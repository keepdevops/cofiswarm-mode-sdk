#pragma once

#include "../agent.h"
#include "../json.hpp"

#include <functional>
#include <string>
#include <unordered_set>
#include <vector>

struct ModeContext {
    const std::vector<Agent>& agents;
    const std::string& user_prompt;
    double temperature;
    const nlohmann::json& mode_config; // per-mode options from swarm-config.json
    bool quality_pass = false;
    std::string quality_pass_target = "programmer"; // agent to re-run on quality pass

    // Token budget context — injected into Foreman/classifier prompt when set.
    int    budget_remaining = -1;   // -1 = unlimited; ≥0 = tokens left this session
    double kv_pressure      = 0.0;  // 0–1 KV cache fill ratio from frontend

    // Per-agent RAG targeting: when non-empty, only named agents receive rag_context_block.
    // When empty, rag_context_block is already baked into user_prompt (legacy path).
    std::string rag_context_block;
    std::unordered_set<std::string> rag_agents;

    // Returns the prompt for a specific agent, prepending RAG context if targeted.
    std::string prompt_for(const std::string& agent_name) const {
        if (rag_context_block.empty() || rag_agents.empty()) return user_prompt;
        if (rag_agents.count(agent_name)) return rag_context_block + user_prompt;
        return user_prompt;
    }
};

// A mode returns an envelope: {mode, agents, final, meta}.
// - agents: {name: text, ...} per-agent outputs
// - final: combined text when the mode produces one, else null
// - meta: mode-specific details (rounds, ordering, errors, etc.)
using ModeFn = std::function<nlohmann::json(const ModeContext&)>;

struct Mode {
    std::string name;        // "flat", "scatter-gather", ...
    std::string description; // one-line, shown in UI
    ModeFn run;
};

namespace modes {

// Detect call_agent error markers. call_agent returns the response text on
// success and one of three sentinel strings on failure:
//   "[<name> error] ..."
//   "Agent <name> (Port N) is not responding."
//   "Connection Error (<name>): ..."
// Modes use this to record per-stage failures in meta.errors[] without having
// to introspect the message format at call sites.
inline bool is_error_response(const std::string& text, const std::string& agent_name) {
    if (text.empty()) return true;
    if (text.rfind("[" + agent_name + " error]", 0) == 0) return true;
    if (text.rfind("Connection Error (" + agent_name + ")", 0) == 0) return true;
    if (text.find(" is not responding.") != std::string::npos
        && text.rfind("Agent " + agent_name, 0) == 0) return true;
    return false;
}

// Register a mode. Safe to call from static initializers before main().
void register_mode(const Mode& m);

// Lookup. Returns nullptr if name is unknown.
const Mode* get(const std::string& name);

// Snapshot of all registered modes, in registration order.
std::vector<Mode> list();

// Set the active mode. Returns false if name is not registered.
bool set_active(const std::string& name);

// Name of the active mode (defaults to first registered mode if none set).
std::string active();

} // namespace modes
