### Metris

#### Description
Metris scrapes all Kyma clusters and uses Shoot information to generate metrics as event streams. The generated event streams are POST-ed to an events collecting system.

#### Environment variables


#### Flags


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