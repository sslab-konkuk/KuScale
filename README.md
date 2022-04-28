# KuScale
Konkuk University Auto Scaler Using KubeShare GPU virtualizatiion 


## Deploy Custom Kubeshare 
From https://github.com/guswns531/KubeShare.git and https://github.com/guswns531/Gemini.git

Fixed for environments like k8s v1.23.1, Gemini 2.0 support and exporting total usage.
```
kubectl apply -f ./kubeshare-deploy
```

## Complie KusScale 

GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go get -u k8s.io/client-go@v0.17.2 github.com/googleapis/gnostic@v0.3.1 golang.org/x/net@v0.0.0-20191004110552-13f9640d40b9 ./...

## Delete Custom Kubeshare
```
kubectl delete -f ./kubeshare-deploy
```