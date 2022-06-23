# Install Knative with knative


## Installing Knative Serving using YAML files

### Prerequisites
For production purposes, it is recommended that:
- You have a cluster that uses Kubernetes v1.22 or newer (we currently use k8s v1.23.1)

### Install the Knative Serving component¶
```
cd deploy/knative

wget https://github.com/knative/serving/releases/download/knative-v1.5.0/serving-crds.yaml
wget https://github.com/knative/serving/releases/download/knative-v1.5.0/serving-core.yaml

kubectl apply -f deploy/knative/serving-crds.yaml
kubectl apply -f deploy/knative/serving-core.yaml 
```

### Install a networking layer
Kourier (Choose this if you are not sure)
```
wget https://github.com/knative/net-kourier/releases/download/knative-v1.5.0/kourier.yaml
```
```
vi deploy/knative/kourier.yaml
apiVersion: v1
kind: Service
metadata:
  name: kourier
  namespace: kourier-system
  labels:
    networking.knative.dev/ingress-provider: kourier
    app.kubernetes.io/component: net-kourier
    app.kubernetes.io/version: "1.5.0"
    app.kubernetes.io/name: knative-serving
spec:
  loadBalancerIP: 192.168.100.35 # this apply
  ports:
    - name: http2
      port: 80
      protocol: TCP
      targetPort: 8080
    - name: https
      port: 443
      protocol: TCP
      targetPort: 8443
  selector:
    app: 3scale-kourier-gateway
  type: LoadBalancer

```


```
kubectl apply -f deploy/knative/kourier.yaml

# Configure Knative Serving to use Kourier by default by running the command:

kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress-class":"kourier.ingress.networking.knative.dev"}}'

# Fetch the External IP address or CNAME by running the command:

kubectl --namespace kourier-system get service kourier

NAME      TYPE           CLUSTER-IP       EXTERNAL-IP      PORT(S)                      AGE
kourier   LoadBalancer   10.110.103.243   192.168.100.35   80:32226/TCP,443:31090/TCP   4m9s
```

> ### Kourier
>#### https://developers.redhat.com/blog/2020/06/30/kourier-a-lightweight-knative-serving-ingress#what_is_kourier_
>>Until recently, Knative Serving used Istio as its default networking component for handling external cluster traffic and service-to-service communication. Istio is a great service mesh solution, but it can add unwanted complexity and resource use to your cluster if you don't need it.
>>To simplify the ingress side of Knative Serving. Knative recently adopted Kourier, so it is now a part of the Knative family! This article introduces Kourier and gets you started with using it as a simpler, more lightweight way to expose Knative applications to an external network.
>Like Istio, Kourier is a lightweight ingress based on the Envoy gateway with no additional custom resource definitions (CRDs). It is composed of two parts:
>- The Kourier gateway is Envoy running with a base bootstrap configuration that connects back to the Kourier control plane.
>- The Kourier control plane handles Knative ingress objects and keeps the Envoy configuration up to date.

### Verify the installation
```
kubectl get pods -n knative-serving

NAME                                      READY   STATUS    RESTARTS   AGE
activator-67688f67c6-scx4m                1/1     Running   0          8m3s
autoscaler-58f7dfdb67-2htc8               1/1     Running   0          8m3s
controller-6ddd5b667d-cjht6               1/1     Running   0          8m3s
domain-mapping-9657f967f-l98g8            1/1     Running   0          8m3s
domainmapping-webhook-f5bfc7479-s7lzm     1/1     Running   0          8m3s
net-kourier-controller-65f84df67b-26d9h   1/1     Running   0          22s
webhook-5c4bff9565-5tvng                  1/1     Running   0          8m3s
```

### Configorue DNS with Magic DNS (sslip.io)

>This will only work if the cluster LoadBalancer Service exposes an IPv4 address or hostname, so it will not work with IPv6 clusters or local setups like minikube unless minikube tunnel is running
```
wget https://github.com/knative/serving/releases/download/knative-v1.5.0/serving-default-domain.yaml

kubectl apply -f serving-default-domain.yaml
```


### Configure Docker registry Login
```
kubectl create secret docker-registry container-registry \
  --docker-server=https://docker.io/ \
  --docker-email=guswns531@gmail.com \
  --docker-username=guswns531 \
  --docker-password=my-gcr-password

kubectl get secret container-registry -o=yaml


docker build -t guswns531/jobs:hello-python-v01 .
docker push guswns531/jobs:hello-python-v01

kubectl get ksvc helloworld-python  --output=custom-columns=NAME:.metadata.name,URL:.status.url
```

### Autoscaling Test
```
git clone -b "release-1.5" https://github.com/knative/docs knative-docs
cd knative-docs
kubectl apply -f docs/serving/autoscaling/autoscale-go/service.yaml


kubectl apply -f test/autoscale-go-ksvc.yaml
sudo apt-get install -y hey
hey -z 30s -c 50 \
  "http://autoscale-go.default.192.168.100.35.sslip.io?sleep=100&prime=10000&bloat=5" \
  && kubectl get pods
  
kubectl delete -f test/autoscale-go-ksvc.yaml

```



### Global settings¶

Global settings for autoscaling are configured using the config-autoscaler ConfigMap. If you installed Knative Serving using the Operator, you can set global configuration settings in the spec.config.autoscaler ConfigMap, located in the KnativeServing custom resource (CR).

EXAMPLE OF THE DEFAULT AUTOSCALING CONFIGMAP¶
```
kubectl -n knative-serving describe configmaps config-autoscaler
# or 
apiVersion: v1
kind: ConfigMap
metadata:
 name: config-autoscaler
 namespace: knative-serving
data:
 container-concurrency-target-default: "100"
 container-concurrency-target-percentage: "0.7"
 enable-scale-to-zero: "true"
 max-scale-up-rate: "1000"
 max-scale-down-rate: "2"
 panic-window-percentage: "10"
 panic-threshold-percentage: "200"
 scale-to-zero-grace-period: "30s"
 scale-to-zero-pod-retention-period: "0s"
 stable-window: "60s"
 target-burst-capacity: "200"
 requests-per-second-target-default: "200"
````


### About Autoscale 

https://github.com/knative/serving/blob/main/docs/scaling/SYSTEM.md