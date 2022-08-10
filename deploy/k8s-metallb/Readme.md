# SetUp k8s-metallb with Cailco and MetalLB

```
sudo su
sudo apt-get install -y ipset
sudo kubeadm reset
sudo kubeadm init --pod-network-cidr=192.168.0.0/16

mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config

kubectl get configmap kube-proxy -n kube-system -o yaml | sed -e "s/strictARP: false/strictARP: true/" | sed -e "s/mode: \"""/mode: \"ipvs/" | kubectl diff -f - -n kube-system
kubectl get configmap kube-proxy -n kube-system -o yaml | sed -e "s/strictARP: false/strictARP: true/" | sed -e "s/mode: \"""/mode: \"ipvs/" | kubectl apply -f - -n kube-system

kubectl -n kube-system delete pod kube-proxy-<id>
kubectl -n kube-system logs kube-proxy-<id>

wget https://projectcalico.docs.tigera.io/manifests/tigera-operator.yaml
wget https://projectcalico.docs.tigera.io/manifests/custom-resources.yaml

kubectl apply -f deploy/k8s-metallb/tigera-operator.yaml
kubectl apply -f deploy/k8s-metallb/custom-resources.yaml

kubectl taint nodes --all node-role.kubernetes.io/master-
kubectl get pods --all-namespaces

wget https://raw.githubusercontent.com/metallb/metallb/v0.12.1/manifests/namespace.yaml
wget https://raw.githubusercontent.com/metallb/metallb/v0.12.1/manifests/metallb.yaml

kubectl apply -f deploy/k8s-metallb/namespace.yaml
kubectl apply -f deploy/k8s-metallb/metallb.yaml

```

```
vi deploy/k8s-metallb/metallb-config.yaml

apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - 192.168.100.35-192.168.100.39 # use ip addresses in your network

kubectl apply -f deploy/k8s-metallb/metallb-config.yaml
```

### MetalLB
#### https://metallb.universe.tf/installation/
MetalLB is a load-balancer implementation for bare metal Kubernetes clusters, using standard routing protocols
- Bare-metal cluster operators are left with two lesser tools to bring user traffic into their clusters, “NodePort” and “externalIPs” services. Both of these options have significant downsides for production use, which makes bare-metal clusters second-class citizens in the Kubernetes ecosystem.



kubectl delete -f deploy/k8s-metallb/metallb-config.yaml
kubectl delete -f deploy/k8s-metallb/metallb.yaml
kubectl delete -f deploy/k8s-metallb/namespace.yaml

