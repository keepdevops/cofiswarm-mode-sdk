#include "mode_module.h"
#include "mode_module_policy.h"

#include <unordered_set>

namespace mode_module {

using json = nlohmann::json;

std::string option_string(const json& cfg, const std::string& key, const std::string& fallback) {
    return cfg.contains(key) && cfg[key].is_string() ? cfg[key].get<std::string>() : fallback;
}

json module_meta(const std::string& module_name, const json& cfg) {
    json out = {{"module", module_name}, {"standalone", true}};
    for (const auto& key : {"variant_policy", "preset", "synthesis_policy", "classifier_policy"}) {
        if (cfg.contains(key) && cfg[key].is_string()) out[key] = cfg[key];
    }
    return out;
}

std::string flat_prompt_for_agent(const std::string& user_prompt, const Agent& agent,
                                   const std::string& variant_policy, size_t index, size_t total) {
    if (variant_policy.empty() || variant_policy == "standard") return user_prompt;
    if (variant_policy == "distinct") {
        return "Flat variant policy: produce an independent solution from your role. "
               "Avoid copying the obvious first approach if another reasonable design exists. "
               "Variant " + std::to_string(index + 1) + " of " + std::to_string(total)
               + " from agent '" + agent.name + "'.\n\nUser request:\n" + user_prompt;
    }
    if (variant_policy == "code-alternatives") {
        return "Flat variant policy: if this is a coding task, produce a complete "
               "alternative implementation strategy with build/run notes. State tradeoffs. "
               "Do not merely summarize.\n\nUser request:\n" + user_prompt;
    }
    return user_prompt;
}

std::vector<std::string> agents_with_tag(const std::vector<Agent>& agents, const std::string& tag) {
    std::vector<std::string> out;
    for (const auto& a : agents)
        for (const auto& t : a.tags)
            if (t == tag) { out.push_back(a.name); break; }
    return out;
}

std::string first_with_tag(const std::vector<Agent>& agents, const std::string& tag) {
    for (const auto& a : agents)
        for (const auto& t : a.tags)
            if (t == tag) return a.name;
    return {};
}

std::vector<std::string> pipeline_preset_order(const std::string& preset, const std::vector<Agent>& agents) {
    return mode_policy::pipeline_preset_order(preset, agents);
}

std::string pipeline_stage_prompt(const std::string& staged_prompt, const std::string& agent_name, const std::string& preset) {
    return mode_policy::pipeline_stage_prompt(staged_prompt, agent_name, preset);
}

std::string cascade_synthesis_instruction(const std::string& policy) {
    return mode_policy::cascade_synthesis_instruction(policy);
}

std::string router_policy_instruction(const std::string& policy) {
    return mode_policy::router_policy_instruction(policy);
}

}  // namespace mode_module
