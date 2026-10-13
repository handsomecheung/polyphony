# android-emulator

Android 14 (API 34) Emulator container running with hardware KVM acceleration.
Powered by [budtmo/docker-android](https://github.com/budtmo/docker-android).


## Kubernetes Deployment

Deploy as a Kubernetes Pod pinned to `nur` node:

```bash
./deploy.sh
```

The Service `android-emulator` exposes:
- Port `5555`: ADB (TCP)
