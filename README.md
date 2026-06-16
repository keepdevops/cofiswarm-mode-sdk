# cofiswarm-mode-sdk

Shared mode plugin contract (C++ reference + Go HTTP helpers).

- C++: `include/cofiswarm/mode.h`, `src/registry.cpp` (coordinator legacy)
- Go: `pkg/mode` — config YAML, envelope, HTTP server scaffold for mode repos

## Go usage

```go
srv, _ := mode.NewServer("flat", "/etc/cofiswarm/mode-flat/mode-flat.yaml")
http.ListenAndServe(srv.Addr(), srv.Handler())
```

Config must include `dispatch_url` and `slot_manager_url`.
