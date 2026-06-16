#pragma once

// Prompt/metadata helpers shared across modes (preset strings, variant text).
// The orchestration contract is `struct Mode` + `ModeContext` in modes/mode.h —
// this header does not define the Mode plugin type.

#include "agent.h"
#include "json.hpp"

#include <string>
#include <vector>

namespace mode_module {

std::string option_string(const nlohmann::json& cfg,
                          const std::string& key,
                          const std::string& fallback = "");

nlohmann::json module_meta(const std::string& module_name,
                           const nlohmann::json& cfg);

std::string flat_prompt_for_agent(const std::string& user_prompt,
                                  const Agent& agent,
                                  const std::string& variant_policy,
                                  size_t index,
                                  size_t total);

std::vector<std::string> pipeline_preset_order(const std::string& preset,
                                               const std::vector<Agent>& agents);

std::string pipeline_stage_prompt(const std::string& staged_prompt,
                                  const std::string& agent_name,
                                  const std::string& preset);

std::string cascade_synthesis_instruction(const std::string& policy);

std::string router_policy_instruction(const std::string& policy);

// Returns names of agents that have `tag` in their tags vector, in config order.
std::vector<std::string> agents_with_tag(const std::vector<Agent>& agents,
                                         const std::string& tag);

// Returns the first agent name that has `tag`, or empty string if none.
std::string first_with_tag(const std::vector<Agent>& agents, const std::string& tag);

}  // namespace mode_module
