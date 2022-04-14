# KuScale
Konkuk University Auto Scaler Using KubeShare GPU virtualizatiion 


## Deploy Custom Kubeshare 
From https://github.com/guswns531/KubeShare.git and https://github.com/guswns531/Gemini.git

Fixed for environments like k8s v1.23.1, Gemini 2.0 support and exporting total usage.
```
kubectl apply -f ./kubeshare-deploy
```

## Complie KusScale 


## Delete Custom Kubeshare
```
kubectl delete -f ./kubeshare-deploy
```