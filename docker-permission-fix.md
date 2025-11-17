# Docker Permission Fix Guide

## Problem
When trying to monitor Docker containers, you may encounter this error:
```
Failed to initialize Docker monitor: failed to connect to Docker daemon: permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock: Head "http://%2Fvar%2Frun%2Fdocker.sock/_ping": dial unix /var/run/docker.sock: connect: permission denied
```

## Solutions

### Solution 1: Check Docker Socket Permissions (Recommended)

First, check the permissions on your Docker socket:

```bash
# Check Docker socket permissions
ls -la /var/run/docker.sock

# Expected output should show group write permissions:
# srw-rw---- 1 root docker 0 Nov 17 12:00 /var/run/docker.sock
```

If the group is not `docker` or doesn't have write permissions, you may need to fix it:

```bash
# Add your user to the docker group
sudo usermod -aG docker $USER

# Restart your session or run:
newgrp docker

# Verify you're in the docker group
groups
```

### Solution 2: Use Docker Compose with Correct User Mapping

Update your `docker-compose.yml` to use the correct user ID:

```bash
# Find your user and group IDs
id -u
id -g

# Update docker-compose.yml with your IDs
```

Then uncomment and update the user line in `docker-compose.yml`:
```yaml
user: "1000:1000"  # Replace with your actual user:group IDs
```

### Solution 3: Run Container as Root (Not Recommended for Production)

If the above solutions don't work, you can temporarily run the container as root:

```yaml
# In docker-compose.yml, comment out the user line and use:
privileged: true
```

Or run directly with Docker:
```bash
docker run -d \
  --name monic-monitor \
  --privileged \
  --user root \
  --network host \
  -v /:/host:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  monic-monitor
```

### Solution 4: Fix Docker Socket Group in Container

The updated Dockerfile now includes the docker group, but you may need to ensure the group ID matches:

```bash
# Check the docker group ID on your host
getent group docker

# The output should show something like:
# docker:x:999:username

# If needed, you can modify the Dockerfile to use the specific group ID
```

## Testing the Fix

After applying the fix, test if Docker monitoring works:

1. Rebuild the container:
```bash
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

2. Check the logs:
```bash
docker logs monic-monitor
```

3. Look for successful Docker initialization:
```
Docker monitoring initialized successfully
```

## Alternative: Disable Docker Monitoring

If you don't need Docker monitoring, you can disable it in the configuration:

```json
{
  "docker_checks": {
    "enabled": false,
    "check_interval": 60,
    "containers": []
  }
}
```

## Common Issues

### Issue 1: Docker Group Doesn't Exist in Container
The container needs the `docker` group. The updated Dockerfile now creates this group and adds the `monic` user to it.

### Issue 2: Group ID Mismatch
The docker group ID inside the container must match the host's docker group ID. The current solution creates a new docker group, which should work in most cases.

### Issue 3: SELinux/AppArmor Restrictions
On some systems, additional security policies may block access. You may need to adjust SELinux or AppArmor policies.

## Verification

To verify Docker monitoring is working:

1. Check the application logs for Docker-related messages
2. Access the HTTP stats endpoint and look for Docker container information
3. Ensure the application can list and monitor running containers

If you continue to have issues, please check your system's Docker installation and ensure the Docker daemon is running properly.
