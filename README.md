### Metris

#### Description
Metris scrapes all Kyma clusters and uses Shoot information to generate metrics as event streams. The generated event streams are POST-ed to an events collecting system.

#### Usage

- `Metris` comes with the following command line argument flags:

    | Flag | Description | Default Value   |
    | ----- | ------------ | --------------- |
    | `gardener-secret-path` | The path to the secret which contains kubeconfig of the Gardener MPS cluster. | `/gardener/kubeconfig` |
    | `gardener-namespace` | The namespace in gardener cluster where information on Kyma clusters are. | `garden-kyma-dev`    |
    | `scrape-interval` | The wait duration of the interval between 2 executions of metrics generation. | `3m`         |
    | `worker-pool-size` | The number of workers in the pool. | `5` |
    | `log-level` | The log-level of the application. E.g. fatal, error, info, debug etc. | `info` |
    | `listen-addr` | The application starts server in this port to cater to the metrics and health endpoints. | `8080` |
    | `debug-port` | The custom port to debug when needed. `0` will disable debugging server. | `0` |

- `Metris` comes with the following environment variables:
     
     | Variable | Description | Default Value   |
     | ----- | ------------ | ------------- |
     | `PUBLIC_CLOUD_SPECS` | The specification contains the CPU, Network and Disk information for all machine types from a public cloud provider.  | `-` |
     | `KEB_URL` | The KEB URL where Metris fetches runtime information. | `-` |
     | `KEB_TIMEOUT` | The timeout governs the connections from Metris to KEB | `30s` |
     | `KEB_RETRY_COUNT` | The number of retries Metris will do when connecting to KEB fails. | 5 |
     | `KEB_POLL_WAIT_DURATION` | The wait duration for Metris between each execution of polling KEB for runtime information. | `10m` |
     | `EDP_URL` | The EDP base URL where Metris will ingest event-stream to. | `-` |
     | `EDP_TOKEN` | The token used to connect to EDP. | `-` |
     | `EDP_NAMESPACE` | The namespace in EDP where Metris will ingest event-stream to.| `kyma-dev` |
     | `EDP_DATASTREAM_NAME` | The datastream in EDP where Metris will ingest event-stream to. | `consumption-metrics` |
     | `EDP_DATASTREAM_VERSION` | The datastream version which Metris will use. | `1` |
     | `EDP_DATASTREAM_ENV` | The datastream environment which Metris will use.  | `dev` |
     | `EDP_TIMEOUT` | The timeout for Metris connections to EDP. | `30s` |
     | `EDP_RETRY` | The number of retries for Metris connections to EDP. | `3` |

#### Development
- Run a deployment in currently configured k8s cluster

```
ko apply -f dev/  
```

- Resolve all dependencies
```
make gomod-vendor
```

- Run tests
```
make tests
```

- Run tests and publish a test coverage report
```
make publish-test-results
```

#### Troubleshooting
- Check logs
```
kubectl logs -l app=metrisv2 -n kcp-system -c metrisv2 
```
