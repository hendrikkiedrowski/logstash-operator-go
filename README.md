# logstash-operator-go

rewrite of the famous logstash-operator from bastibrunner/logstash-operator


## Architecture 

- Single controller to avoid issues with parallelism
- Separate out CRDs
  - source CRD
    - create configmap
  - filter CRD
    - create configmap
  - sink CRD
    - create configmap
  - pipeline CRD
    - if all pipeline parts exist:
      - mount source, filter, and sink configmap into logstash
      - additionally create services for all sources
  - logstash CRD
    - reserve resources (i.e., create stateful set etc.)
    - start logstash application
- Operational side
  - Logstash config reloader side car?
  - Multiple pipelines per logstash instance
  - One persistent volume per logstash instance
- Validation
  - Validation web hooks
  - Use logstash validation
  - Add Additional validation logic


## TODOs

- [X] Separate logstash CRD from controller
- [ ] Write tests for logstash CRD reconciler
- [ ] Try out manually mounting a config into a running logstash (or somehow trigger a reload)
- [ ] Create a single pipeline CRD that includes source, filter, and sink

