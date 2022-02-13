## Leader Election

Metacontroller leverages [controller-runtime's leader election](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/leaderelection). 
This is used to ensure that multiple replicas of metacontroller can run with only one active pod, for active-passive high availability.


### Enable leader election
Once enabled, metacontroller will attempt to acquire a leader on startup.
- Add the metacontroller command-line argument leader-election
  ```
  args:
  - --leader-election
  ```
- Increase `replicas` to desired count in [values.yaml](../../deploy/helm/values.yaml)
  ```
  replicas: 2  
  ```     
- See [configuration.md](../../docs/src/guide/configuration.md) for additional configuration arguments.

### Disable leader election
Once disabled, metacontroller will not attempt to acquire a leader on startup.
- Omit the metacontroller command-line argument leader-election or set to false.
  ```
  args:
  - --leader-election=false
  ```

