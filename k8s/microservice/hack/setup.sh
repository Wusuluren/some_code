go get -u github.com/golang/protobuf/protoc-gen-go@v1.3.2
export PATH=$PATH:~/Downloads/protoc-3.19.4-osx-x86_64/bin/

echo "10.211.55.7 registry" >> /etc/hosts

echo '{"insecure-registries":["registry:5000","http://registry:5000","https://registry:5000"]}' >> /etc/docker/daemon.json
systemctl daemon-reload
systemctl restart docker

#docker pull lianyuxue1020/kube-webhook-certgen:v1.1.1
#docker tag lianyuxue1020/kube-webhook-certgen:v1.1.1 k8s.gcr.io/ingress-nginx/kube-webhook-certgen:v1.1.1

#minikube start --image-mirror-country=cn --registry-mirror=https://registry.docker-cn.com --insecure-registry="registry:5000"
minikube start --registry-mirror=registry.cn-hangzhou.aliyuncs.com/google_containers --insecure-registry="registry:5000"
minikube addons enable ingress