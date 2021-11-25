# Log

## 2021-11-26

Statefulset is intialized, but resources are not requested correctly.

```
$> kubectl -n logstash-operator-go-system describe statefulset.apps/logstash-sample
[...]
  Type     Reason        Age               From                    Message
  ----     ------        ----              ----                    -------
  Warning  FailedCreate  2s (x11 over 7s)  statefulset-controller  create Claim logstash-sample-data-logstash-sample-0 for Pod logstash-sample-0 in StatefulSet logstash-sample failed error: PersistentVolumeClaim "logstash-sample-data-logstash-sample-0" is invalid: spec.resources[storage]: Required value
  Warning  FailedCreate  2s (x11 over 7s)  statefulset-controller  create Pod logstash-sample-0 in StatefulSet logstash-sample failed error: failed to create PVC logstash-sample-data-logstash-sample-0: PersistentVolumeClaim "logstash-sample-data-logstash-sample-0" is invalid: spec.resources[storage]: Required value
```