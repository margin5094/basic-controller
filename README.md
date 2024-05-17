# ClusterScan Controller

This project is a Kubernetes controller for managing security scans on a cluster using tools like `kube-bench` and `kube-hunter`. The controller reconciles a custom resource named `ClusterScan`, which specifies the tool to use, the type of job (Job or CronJob), and other relevant configurations.

##

## Documentation

We can also use Blob storage for storing the logs which is scalable option here. All logs can be store on the cloud platform and link to that storage can be made available to status of controller.

Further, We can also add more tools for scanning. We can make the container of the controller and can run it in the cluster.
