#!/bin/bash
#
# Initialize Forgejo after a fresh install or PVC reset.
#
# This script:
#   1. Waits for Forgejo pod to be running
#   2. Fixes RUN_USER in app.ini if needed (OpenShift runs as random UID)
#   3. Creates the devadmin admin user
#   4. Creates the acs-next repo
#

set -euo pipefail

NAMESPACE="forgejo"
ADMIN_USER="devadmin"
ADMIN_PASS="admin123"
ADMIN_EMAIL="devadmin@local"
FORGEJO_URL="http://forgejo.forgejo.svc:3000"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

wait_for_pod() {
    log "Waiting for Forgejo pod to be running..."
    local attempts=0
    while [[ $attempts -lt 60 ]]; do
        local status
        status=$(kubectl get pods -n "$NAMESPACE" -l app=forgejo -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")
        if [[ "$status" == "Running" ]]; then
            log "Pod is running"
            return 0
        fi
        ((attempts++))
        sleep 2
    done
    log "ERROR: Timed out waiting for pod"
    return 1
}

check_pod_crash_loop() {
    local restarts
    restarts=$(kubectl get pods -n "$NAMESPACE" -l app=forgejo -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null || echo "0")
    if [[ "$restarts" -gt 2 ]]; then
        return 0  # In crash loop
    fi
    return 1
}

fix_run_user() {
    log "Fixing RUN_USER in app.ini..."

    # Scale down to release PVC
    kubectl scale deployment -n "$NAMESPACE" forgejo --replicas=0
    sleep 3

    # Wait for pod to terminate
    local attempts=0
    while [[ $attempts -lt 30 ]]; do
        local count
        count=$(kubectl get pods -n "$NAMESPACE" -l app=forgejo --no-headers 2>/dev/null | wc -l)
        if [[ "$count" -eq 0 ]]; then
            break
        fi
        ((attempts++))
        sleep 2
    done

    # Fix the config using a debug pod
    kubectl run -n "$NAMESPACE" forgejo-fix --rm -i --restart=Never \
        --image=busybox \
        --overrides='{
            "spec": {
                "containers": [{
                    "name": "forgejo-fix",
                    "image": "busybox",
                    "command": ["sh", "-c", "sed -i \"s/^RUN_USER = .*$/RUN_USER = /\" /data/custom/conf/app.ini && echo RUN_USER fixed"],
                    "volumeMounts": [{
                        "name": "data",
                        "mountPath": "/data"
                    }]
                }],
                "volumes": [{
                    "name": "data",
                    "persistentVolumeClaim": {
                        "claimName": "forgejo-data"
                    }
                }]
            }
        }'

    # Scale back up
    kubectl scale deployment -n "$NAMESPACE" forgejo --replicas=1

    wait_for_pod
    sleep 5  # Give Forgejo time to start listening
}

wait_for_api() {
    log "Waiting for Forgejo API to be available..."
    local attempts=0
    while [[ $attempts -lt 30 ]]; do
        local code
        code=$(curl -s -o /dev/null -w '%{http_code}' "$FORGEJO_URL/api/v1/version" 2>/dev/null || echo "000")
        if [[ "$code" == "200" ]]; then
            log "API is available"
            return 0
        fi
        ((attempts++))
        sleep 2
    done
    log "ERROR: Timed out waiting for API"
    return 1
}

check_install_page() {
    # Check if Forgejo is showing the install page
    local response
    response=$(curl -s "$FORGEJO_URL/" 2>/dev/null | head -100)
    if echo "$response" | grep -q "Installation"; then
        return 0  # Install page showing
    fi
    return 1
}

user_exists() {
    local code
    code=$(curl -s -o /dev/null -w '%{http_code}' -u "$ADMIN_USER:$ADMIN_PASS" "$FORGEJO_URL/api/v1/user" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        return 0
    fi
    return 1
}

create_admin_user() {
    log "Creating admin user '$ADMIN_USER'..."
    kubectl exec -n "$NAMESPACE" deployment/forgejo -- gitea admin user create \
        --username "$ADMIN_USER" \
        --password "$ADMIN_PASS" \
        --email "$ADMIN_EMAIL" \
        --admin
    log "Admin user created"
}

repo_exists() {
    local repo_name="$1"
    local code
    code=$(curl -s -o /dev/null -w '%{http_code}' -u "$ADMIN_USER:$ADMIN_PASS" "$FORGEJO_URL/api/v1/repos/$ADMIN_USER/$repo_name" 2>/dev/null || echo "000")
    if [[ "$code" == "200" ]]; then
        return 0
    fi
    return 1
}

create_repo() {
    local repo_name="$1"
    log "Creating repo '$repo_name'..."
    local code
    code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$FORGEJO_URL/api/v1/user/repos" \
        -H "Content-Type: application/json" \
        -u "$ADMIN_USER:$ADMIN_PASS" \
        -d "{\"name\": \"$repo_name\", \"private\": false}")
    if [[ "$code" == "201" ]]; then
        log "Repo '$repo_name' created"
    else
        log "ERROR: Failed to create repo (HTTP $code)"
        return 1
    fi
}

main() {
    log "Starting Forgejo initialization..."

    # Wait for initial pod
    wait_for_pod

    # Check if pod is crash-looping (usually due to RUN_USER mismatch)
    sleep 5
    if check_pod_crash_loop; then
        log "Pod appears to be crash-looping, attempting to fix RUN_USER..."
        fix_run_user
    fi

    # Wait for API
    wait_for_api

    # Check if still on install page (shouldn't happen after fix, but just in case)
    if check_install_page; then
        log "ERROR: Forgejo is still showing install page. Manual intervention required."
        log "Visit $FORGEJO_URL to complete installation, then re-run this script."
        exit 1
    fi

    # Create admin user if needed
    if ! user_exists; then
        create_admin_user
    else
        log "Admin user '$ADMIN_USER' already exists"
    fi

    # Create acs-next repo if needed
    if ! repo_exists "acs-next"; then
        create_repo "acs-next"
    else
        log "Repo 'acs-next' already exists"
    fi

    log "Forgejo initialization complete!"
    log ""
    log "Git remote URL: http://$ADMIN_USER:$ADMIN_PASS@forgejo.forgejo.svc:3000/$ADMIN_USER/acs-next.git"
}

main "$@"
