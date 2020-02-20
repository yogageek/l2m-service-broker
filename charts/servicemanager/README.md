# Mongodb Service Manager

Mongodb Service Manager is a mongodb service broker to deploy on-demand mongodb service.

## Introduction

This chart bootstraps a mongodb service manager deployment on a [Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh) package manager.

## Release Note

### Features

* Serve 5 service broker API (GetCatalog, Provision, Deprovision, Bind, Unbind)
* Support MongoDB shared service


## Prerequisites

- Kubernetes 1.14+ 

## Installing the Chart

To install the chart with the release name `mongodb-sm`:

```bash
$ helm repo add --username <username> --password <password> frbimo https://harbor.arfa.wise-paas.com/chartrepo/frbimo/
```
```bash
$ helm install --name <deployment_name> --namespace <namespace> frbimo/mongodb-sm --version 0.1.2
```

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `mongodb-sm` deployment

```bash
$ helm delete <deployment_name> --purge
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

## Configuration

Default value of values.yaml

| Parameter                                    | Description                                                                     | Default                                              |
| -------------------------------------------- | ------------------------------------------------------------------------------- | ---------------------------------------------------- |
| `image.repository`                           | Image repository                                                               | `harbor.arfa.wise-paas.com/frbimo/mongodb-sm`    |
| `image.tag`                                  | Image tag                                                                      | `v1.0.0-34-ge7fdc73`                                             |
| `image.pullPolicy`                           | Image pullPolicy                                                               | `Always`                                       |
| `image.pullSecrets.enabled`                           | Enabled Kubernetes ImagePullSecrets                                                               |`false`                                       |
| `image.pullSecrets.imagePullSecrets`                           | Kubernetes ImagePullSecrets                                                               |`mongodb-sm-regcred`                                       |
| `cronjob.enabled`                           | enable cronjob                                                               | `true`    |
| `cronjob.repository`                           | cronjob repository                                                               | `harbor.arfa.wise-paas.com/frbimo/mongodb-cronjob`    |
| `cronjob.tag`                           | cronjob tag                                                              | `v1.0.0-34-ge7fdc73`    |
| `replicas`                               | Pod replicas                                                                | `1`                                                  |
| `service.type`                               | Service Type                                                                  | `NodePort`                                          |
| `service.port`                               | Service port                                                                   | `8000`                                               |
| `insecure`                                   | run without TLS                                                                       | `true`                                                 | 
| `deprovision`                                   | enable deprovision                                                                       | `false`                                                 | 
| `authenticate`                                   | k8s token authentication                                                                       | `false`                                                 | 
| `deployClusterServiceBroker`                                | service catalog ClusterServiceBroker                                                                     | `false`                                                 |
| `verbose.level`                                   | log level  <0:Error> <2:Info>                                                                      | `0`                                                 |
| `resources`                       | Resource limit & request                                                             | `{}`                         |
| `env.enabled`                       | Enable environment variable                                                             | `true`                         |
| `env.configmap`                       | ENV variable                                                             | `{}`                         |
Service manager config 

| Parameter                                    | Description                                                                     | Default                                              |
| -------------------------------------------- | ------------------------------------------------------------------------------- | ---------------------------------------------------- |
| `MONGODB_HOST1`                               | Mongodb host 1                                                                  | `mongodb-mongodb-replicaset-svc-0.wise-paas-db.svc.cluster.local`                                      |
| `MONGODB_HOST2`                              | Mongodb host 2                                                                  | `mongodb-mongodb-replicaset-svc-1.wise-paas-db.svc.cluster.local`                                      |
| `MONGODB_HOST3`                              | Mongodb host 3                                                                  | `mongodb-mongodb-replicaset-svc-2.wise-paas-db.svc.cluster.local`                                      |
| `MONGODB_PORT1`                               | Mongodb host 1                                                                  | `27017`                                              |
| `MONGODB_PORT2`                              | Mongodb host 2                                                                  | `27017`                                              |
| `MONGODB_PORT3`                              | Mongodb host 3                                                                  | `27017`                                              |
| `MONGODB_USERNAME`                           | Mongodb username                                                                | `root`                                               |
| `MONGODB_PASSWORD`                           | Mongodb password                                                                | `root`                                               |
| `PSEUDO_ID`            | pseudo ID                                                  | `abc-123`                                               |
| `SHARED_USE`                           | Mongodb service shared status                                                    | `true`  
| `DATABASE`                             | Mongodb auth db                                                                 | `admin`                                              |
| `OPS_PG_HOST1`                       | Postgresql OPS database host                                             | `10.233.13.209`                                            |
| `OPS_PG_PORT1`                         | Postgresql OPS database port                                                 | `5432`                                                 |
| `OPS_PG_USERNAME`                               | Postgresql OPS database username                                                     | `opsaccount`               |
| `OPS_PG_PASSWORD`                             | Postgresql OPS database password                                                           | `cXPG0vZDdZ`                                              |
| `MAX_INSTANCE_PER_DB`                               | maximum number of instance in a single target database                                                     | `40`               |
| `MAX_DAYS_PREDELETION_PERIOD`                             | maximum days of a instance remain before doing complete removal                                                           | `1`                                              |
| `API_USERNAME`                               | SM API username                                                     | `admin`               |
| `API_PASSWORD`                             | SM API password                                                           | `admin`                                              |



> **Tip**: You can use the default [values.yaml](values.yaml)