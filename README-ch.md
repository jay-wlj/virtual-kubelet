# Virtual Kubelet概述

Virtual Kubelet(VK)是一个开源的Kubernetes kubelet的实现，伪装成一个kubelet,目的是将Kubernetes连接到其他API，这允许Kubernetes节点由其他服务支持，例如无服务器容器平台(如ACI、AWS Fargate、Hyper.sh、IoT Edge等)。VK具有可插拔的体系结构，可直接使用Kubernetes原语，使其更容易构建。Virtual Kubelet由Cloud Native Computing Foundation（CNCF）托管。如果您是一家希望帮助塑造容器打包、动态调度和面向微服务的技术发展的公司，请考虑加入CNCF。有关谁参与以及Virtual Kubelet扮演角色的详细信息，请阅读Virtual Kubelet CNCF项目建议书（https://github.com/cncf/toc/blob/master/proposals/virtualkubelet.adoc）。


#### 主要内容

* [工作原理](#工作原理)
* [用法](#usage)
    + [使用集群外实现的外部云供应商](#使用集群外实现的外部云供应商)
    + [使用集群内实现的mock供应商](#使用集群内实现的mock供应商)
* [Providers的实现](#Providers的实现)
    + [目前已有的virtual-kubelet云供应商](#目前已有的virtual-kubelet云供应商)
    + [添加自己的virtual-kubelet供应商](#添加自己的virtual-kubelet供应商)
* [测试](#测试)
    + [单元测试](#单元测试)
    + [本地k8s集群内测试](#本地k8s集群内测试)
* [注意事项](#注意事项)
    + [vk会丢失服务的负载均衡IP地址](#vk会丢失服务的负载均衡IP地址)
    + [判断pod是否运行在virtual-kubelet上](#判断pod是否运行在virtual-kubelet上)

## 工作原理

一般来讲，Kubernetes kubelet为每个Kubernetes节点（Node）实现Pod和容器操作。它们作为每个节点上的代理运行，无论该节点是物理服务器还是虚拟机，并在该节点上处理Pod/容器操作。kubelets将名为PodSpec的配置作为输入，并确保PodSpec中指定的容器正在运行且运行正常。
从Kubernetes API服务器的角度来看，Virtual Kubelet看起来像普通的kubelet，但其关键区别在于它们在其他地方调度容器，例如在云无服务器API中，而不是在节点上。

下面显示了一个Kubernetes集群，其中包含一系列标准kubelet和一个Virtual Kubelet：
![diagram](website/static/img/diagram.svg)

## 用法

可以使用virtual-kubelet命令行工具在k8s集群内部或外部中部署virtual-Kubele。如果在k8s集群中运行virtual-Kubelet，也可以使用[Helm](# Helm)部署它。
使用"--help"查看相关命令参数
```bash
virtual-kubelet --help
```
### 使用集群外实现的外部云供应商
1.选择外部提供商，如aws,azure等
```bash
virtual-kubelet --provider aws
```
当virtual-kubelet被部署后，可以查看k8s集群中新增node信息
```bash
kubectl get nodes
```
此时，能看到新增加的名为"virtual-kubelet"的Node(节点名称可以在部署时指定 --nodename参数,默认为"virtual-kubelet")


### 使用集群内实现的mock供应商
一、注意事项:
1.可以在k8s集群中以pod的形式运行virtual kubelet,此时仅支持已实现的"mock"供应商
2.为了部署virtual kubelet,需要先安装[Skaffold](https://skaffold.dev/), 是一个k8s开发工具,同时需要确保当前k8s环境是'minikube'还是'docker-for-desktop'

二、部署流程
1. 安装minikube，并启动集群环境
```
minikube start --driver=none --kubernetes-version='v1.18.2'
```

1. 克隆virtual-kubelet源码
```bash
git clone https://github.com/virtual-kubelet/virtual-kubelet
cd virtual-kubelet
```
2. 设置skaffold的命令配置参数
```
skaffold config set --kube-context='minikube' local-cluster true
```
3. 运行e2e mock组件
```
make e2e
```
4. 此时可以在k8s中查看该pods及node信息,会看到该virtual-kubelet的pod及虚拟node节点信息
```
[root@node1 virtual-kubelet]# kubectl get pods -o wide
NAME              READY   STATUS    RESTARTS   AGE     IP           NODE    NOMINATED NODE   READINESS GATES
vkubelet-mock-0   1/1     Running   0          3m19s   172.17.0.4   node1   <none>           <none>

[root@node1 virtual-kubelet]# kubectl get nodes
NAME              STATUS   ROLES    AGE     VERSION
node1             Ready    master   5h6m    v1.18.2
vkubelet-mock-0   Ready    agent    2m48s   v1.15.2-vk-v1.2.1-dev
```
5. 待该node在Ready状态下后，可以部署相应的pod到该node上运行，验证virtual-kubelet功能
如，部署nginx的pod到该virutal-node下运行，yaml如下:
```
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  containers:
  - name: nginx
    image: nginx
    imagePullPolicy: IfNotPresent
    ports:
    - containerPort: 80
  nodeSelector:
    type: virtual-kubelet
  tolerations:
  - operator: "Exists"
```
部署该nginx.yaml
```
[root@node1 ~]# kubectl create -f nginx.yml 
pod/nginx created
```

6. 查看该node下的pods运行信息,即可看到nginx的pod运行在
```
[root@node1 ~]# kubectl get pods -o wide
NAME              READY   STATUS    RESTARTS   AGE     IP           NODE              NOMINATED NODE   READINESS GATES
nginx             1/1     Running   0          18s     5.6.7.8      vkubelet-mock-0   <none>           <none>
vkubelet-mock-0   1/1     Running   0          4m10s   172.17.0.4   node1             <none>           <none>
```

## 当前vritual-kubelet的功能

- create, delete and update pods
- container logs, exec, and metrics
- get pod, pods and pod status
- capacity
- node addresses, node capacity, node daemon endpoints
- operating system
- bring your own virtual network


## Providers的实现

此项目具有可插入provider接口，开发人员可以实现该接口，创建具体的vritual-kubelet实例。
特点是virtual-kubelet支持按需及几乎瞬时的容器计算，由k8s管理而不用管理VM基础设施

要实现vkrutal-kubelet, 实现者必须实现以下功能。
1. 提供后端管道，以支持在K8s上下文中对pods、容器和支持资源的生命周期管理。
2. 实现virtual-kubelet的相关接口
3. 不能直接访问Kubernetes API服务器的权限，但具有定义良好的回调机制来获取机密或configmaps等数据

### 目前已有的virtual-kubelet云供应商
1. [Alibaba Cloud ECI Provider](https://github.com/virtual-kubelet/alibabacloud-eci/blob/master/README.md)
2. [Azure Container Instances Provider](https://github.com/virtual-kubelet/azure-aci/blob/master/README.md)
3. [Azure Batch GPU Provider](https://github.com/virtual-kubelet/azure-batch/blob/master/README.md)
4. [AWS Fargate Provider](https://github.com/virtual-kubelet/aws-fargate)
5. [HashiCorp Nomad Provider](https://github.com/virtual-kubelet/nomad/blob/master/README.md)
6. [OpenStack Zun Provider](https://github.com/virtual-kubelet/openstack-zun/blob/master/README.md)


### 添加自己的virtual-kubelet供应商

添加自己的供应商需要以下三个接口。
1. PodLifecylceHandler
2. PodNotifier
3. NodeProvider

#### PodLifecylceHandler

从kubernetes中创建、更新或删除pods等操作时，会调用实现此接口的相应方法。

[godoc#PodLifecylceHandler](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#PodLifecycleHandler)

```go
type PodLifecycleHandler interface {
    // CreatePod takes a Kubernetes Pod and deploys it within the provider.
    CreatePod(ctx context.Context, pod *corev1.Pod) error

    // UpdatePod takes a Kubernetes Pod and updates it within the provider.
    UpdatePod(ctx context.Context, pod *corev1.Pod) error

    // DeletePod takes a Kubernetes Pod and deletes it from the provider.
    DeletePod(ctx context.Context, pod *corev1.Pod) error

    // GetPod retrieves a pod by name from the provider (can be cached).
    GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)

    // GetPodStatus retrieves the status of a pod by name from the provider.
    GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error)

    // GetPods retrieves a list of all pods running on the provider (can be cached).
    GetPods(context.Context) ([]*corev1.Pod, error)
}
```

#### PodNotifier
是一个可选的接口，可以让供应商异步通知virtual-kubelet的有关pod状态更改，如果未实现该接口，vk将会定期检查所有pods的状态。如果要在vk上运行大量的pods，强列建议实现该接口。

[godoc#PodNotifier](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#PodNotifier)

```go
type PodNotifier interface {
    // NotifyPods instructs the notifier to call the passed in function when
    // the pod status changes.
    //
    // NotifyPods should not block callers.
    NotifyPods(context.Context, func(*corev1.Pod))
}
```

`PodLifecycleHandler`是由`PodController`的核心控制逻辑调用的，主要管理分配pods到node的逻辑等
```go
	pc, _ := node.NewPodController(podControllerConfig) // <-- instatiates the pod controller
	pc.Run(ctx) // <-- starts watching for pods to be scheduled on the node
```

#### NodeProvider

NodeProvider负责通知virtual-kubelet关于节点状态的信息更新。vk将定期检查节点的状态相应地更新到Kubernetes。

[godoc#NodeProvider](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#NodeProvider)

```go
type NodeProvider interface {
    // Ping checks if the node is still active.
    // This is intended to be lightweight as it will be called periodically as a
    // heartbeat to keep the node marked as ready in Kubernetes.
    Ping(context.Context) error

    // NotifyNodeStatus is used to asynchronously monitor the node.
    // The passed in callback should be called any time there is a change to the
    // node's status.
    // This will generally trigger a call to the Kubernetes API server to update
    // the status.
    //
    // NotifyNodeStatus should not block callers.
    NotifyNodeStatus(ctx context.Context, cb func(*corev1.Node))
}
```

vk提供了一个基本的nodeprovider，不用自定义node代理的话可以使用它，由在k8s中管理node对象的`NodeController`使用。

[godoc#NaiveNodeProvider](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node#NaiveNodeProvider)

```go
	nc, _ := node.NewNodeController(nodeProvider, nodeSpec) // <-- instantiate a node controller from a node provider and a kubernetes node spec
	nc.Run(ctx) // <-- creates the node in kubernetes and starts up he controller
```

#### API 接口端点

Kubelet的功能之一是接受来自API服务器的请求比如" kubectl日志"和" kubectl执行", 实现的功能在[here](https://godoc.org/github.com/virtual-kubelet/virtual-kubelet/node/api)

## 测试

### 单元测试
在virtual-kubelet目录下执行以下命令进行单元测试
```
make test
```
### 本地k8s集群内测试
在minikube集群环境下，执行以下命令后，会多出一个名为`vkubelet-mock-0`的虚拟节点

```
make e2e
```

## 注意事项

### vk会丢失服务的负载均衡IP地址

#### 代理商不支持服务发现

Kubernetes 1.9及以上版本中为`Controller Manager`中引入了新的flag`ServiceNodeExclusion`, 启用这个标记会允许k8s排除添加到负载均衡池中的虚拟node,同时也能使用外部IP创建面向公共服务。
在`Controller Manager`的manifest中添加命令行参数`--feature-gates=ServiceNodeExclusion=true` 来启用这个特性


### 判断pod是否运行在virtual-kubelet上
可以根据部署virtual-kubelet时的ServiceAccount的集群角色名来判断pod是否运行在其上、
1. 查看具体pod的运行Node
```
[root@node1 ~]# kubectl describe pods nginx
Name:         nginx
Namespace:    default
Priority:     0
Node:         vkubelet-boc/1.2.3.4
****
```
2. 查看该Node的详情信息，节点Role为agent，即被认为是virutal-kubelet代理节点
```
[root@node1 ~]# kubectl describe nodes vkubelet-boc
Name:               vkubelet-boc
Roles:              agent
Labels:             alpha.service-controller.kubernetes.io/exclude-balancer=true
                    beta.kubernetes.io/os=linux
                    kubernetes.io/hostname=vkubelet-boc
                    kubernetes.io/os=linux
                    kubernetes.io/role=agent
                    type=virtual-kubelet
****
```





