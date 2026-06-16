#pragma once
#include "mode_module.h"
#include <string>
#include <vector>

namespace mode_policy {

inline std::vector<std::string> pipeline_preset_order(
    const std::string& preset, const std::vector<Agent>& agents) {
    if (preset.empty()) return {};
    const std::string planner  = mode_module::first_with_tag(agents, "planning");
    const std::string coder    = mode_module::first_with_tag(agents, "coding");
    const std::string reviewer = mode_module::first_with_tag(agents, "review");

    std::vector<std::string> out;
    auto push = [&](const std::string& n) { if (!n.empty()) out.push_back(n); };

    if (preset == "code-quality") {
        push(planner); push(coder); push(reviewer); push(coder);
    } else if (preset == "debug-fix") {
        push(reviewer); push(coder); push(reviewer);
    } else if (preset == "docs-finalize") {
        push(coder);
        push(mode_module::first_with_tag(agents, "synthesis"));
    }
    return out;
}

inline std::string pipeline_stage_prompt(
    const std::string& staged_prompt,
    const std::string& agent_name,
    const std::string& preset) {
    if (preset.empty()) return staged_prompt;
    std::string prefix = "Pipeline preset '" + preset + "'. ";
    if (preset == "code-quality") {
        if (agent_name == "tester")
            prefix += "Audit the previous implementation for compile errors, logic bugs, missing files, unsafe types, and prompt mismatch. Return concrete findings.";
        else if (agent_name == "programmer")
            prefix += "Produce or revise complete runnable code. If prior tester findings exist, fix them directly and return a replacement.";
        else
            prefix += "Plan the implementation with concrete file/module guidance.";
    } else if (preset == "debug-fix") {
        prefix += "Focus on reproducing, isolating, and fixing defects.";
    } else if (preset == "docs-finalize") {
        prefix += "Focus on complete implementation notes, usage docs, and handoff clarity.";
    }
    return prefix + "\n\n" + staged_prompt;
}

inline std::string cascade_synthesis_instruction(const std::string& policy) {
    if (policy == "full-code")
        return "Synthesis policy: preserve complete source files from the best response. Fix obvious compile issues, but do not compress code into summaries.\n\n";
    if (policy == "best-answer-plus-fixes")
        return "Synthesis policy: choose the strongest answer, merge only concrete fixes from other agents, and produce one corrected final answer.\n\n";
    if (policy == "tradeoff-comparison")
        return "Synthesis policy: compare tradeoffs first, then recommend one path.\n\n";
    return "";
}

inline std::string router_policy_instruction(const std::string& policy) {
    if (policy == "code")   return "Classifier policy: prefer planning-role and coding-role agents for implementation tasks. ";
    if (policy == "debug")  return "Classifier policy: prefer coding-role and review-role agents for debugging or correction tasks. ";
    if (policy == "docs")   return "Classifier policy: prefer synthesis-role agents for documentation and summary tasks. ";
    if (policy == "ops")    return "Classifier policy: prefer review-role agents for verification and operations tasks. ";
    return "";
}

}  // namespace mode_policy
