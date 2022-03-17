# logstash-operator-go

rewrite of the famous logstash-operator from bastibrunner/logstash-operator

## TODOs

- Separate out CRDs for logstash pipeline
    - Add CRD configmap for source (creates configmap)
    - Add CRD configmap for filter (creates configmap)
    - Add CRD configmap for sink (creates configmap)
    - Add CRD configmap for pipeline (creates configmap)
    - Generate a single read-only logstash configmap per pipeline
- Operational side
    - Logstash config reloader side car?
    - Multiple pipelines per logstash instance
    - One persistent volume per logstash instance
- Validation
    - Validation web hooks
    - Use logstash validation
    - Add Additional validation logic
