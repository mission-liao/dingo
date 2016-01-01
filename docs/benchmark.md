##Benchmark

All these benchmarks are done in a single node (my notebook) with these profiles:
```bash
Mem: 8G
CPU: 2.66 GHz Intel Core 2 Duo
```
The main idea is to test the speed of task delivery, therefore the worker function just returns what inputs.

###Local Mode
```bash
Macintosh:dingo ml$ go test -bench=Local -run=XXX -cpu=4
PASS
BenchmarkLocal-4       10000        228250 ns/op
ok      github.com/mission-liao/dingo   2.334s
```
