# Orchestration modes (`src2/modes`)

Modes implement `modes::register_mode` ([mode.h](mode.h)) and return envelopes `{ mode, agents, final, meta }`.

## Build artifact

`scripts/build_cpp_binaries.sh` compiles every `*.cpp` here into **`build/libmatrix_modes.a`** and links it into the coordinator with **whole-archive** semantics (`-force_load` on macOS, `--whole-archive` on Linux). That keeps **static initializers** that register each mode from being dropped by the linker.

Coordinator HTTP, dispatch, agents, `mode_module`, and synthesis helpers stay **outside** this archive — modes depend on them, not the reverse.
