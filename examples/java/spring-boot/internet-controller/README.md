# Internet Controller (MetaController) function

This project was created to demonstrate how to use MetaController and Knative Functions. This controller shows how to monitor the internet and trigger some internet production tests only when the application is healthy. 

This function doesn't require any Kubernetes API Server access, hence it is much easier to code and deploy compared to a normal controller.
The downside, or maybe the advantage, is that we are encouraged more to interact with the data plane instead of the control plane.

For this example, the way to check if the internet is healthy is by sending HTTP requests to different websites, instead of relying on Kubernetes Resources. 

To create the Deployment that will run the internet ready production tests, we use MetaController children definitions. 

To run this project you need: 
- `helm` version 3.8+ installed 
- MetaController installed
  - to install with helm you can run the following commands
  - `HELM_EXPERIMENTAL_OCI=1 helm pull oci://ghcr.io/metacontroller/metacontroller-helm --version=v2.2.5` to fetch the metacontroller chart
  - `kubectl create ns metacontroller` Create a namespace
  - `helm install metacontroller metacontroller-helm-v2.2.5.tgz --namespace metacontroller` install the metacontroller chart
- Knative Serving installed
  - Follow the instructions at https://knative.dev
- `func` CLI installed

Once you have this setup, there are two main things to do
- Deploy the function, and you can do this by running `func deploy` at the root of this directory, this will build, publish and deploy the container image as a Knative Service
- Configure the metacontroller to monitor a CRD and then notify our function when a new resource is created. This requires two things: 
  - Create a CRD with the type that we want to reconcile, for this example is the `Internet` resource which lives inside the `metacontroller.github.com` group and that we can apply by running `kubectl apply -f config/crd.yaml`
  - Define a MetaController CompositeController where we define that we want to monitor `Internet` resources, we specify which kind of children these resources can have (in this case Deployments) and where (URL) is the function that will do the reconciliation is. You can create these CompositeController by running `kubectl apply -f config/controller.yaml`


  