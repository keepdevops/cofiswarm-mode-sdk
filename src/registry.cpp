#include "mode.h"

#include <iostream>
#include <mutex>
#include <vector>

namespace {

// Registry state. Insertion order is preserved so list() is stable across
// startups (the UI uses it to populate the mode dropdown).
std::vector<Mode>& registry() {
    static std::vector<Mode> r;
    return r;
}

// Guards the active-mode name. Contention is trivial (rare writes, cheap reads)
// so a plain mutex is sufficient.
std::mutex& active_mutex() {
    static std::mutex m;
    return m;
}

std::string& active_name() {
    static std::string n;
    return n;
}

} // namespace

namespace modes {

void register_mode(const Mode& m) {
    for (const auto& existing : registry()) {
        if (existing.name == m.name) {
            std::cerr << "[coordinator] duplicate mode registration ignored: " << m.name << std::endl;
            return;
        }
    }
    registry().push_back(m);
    std::cout << "🧩 registered mode: " << m.name << " — " << m.description << std::endl;

    // First mode registered becomes the default active mode. Overridden later
    // by coordinator config's default_mode if present.
    std::lock_guard<std::mutex> lock(active_mutex());
    if (active_name().empty()) {
        active_name() = m.name;
    }
}

const Mode* get(const std::string& name) {
    for (const auto& m : registry()) {
        if (m.name == name) return &m;
    }
    return nullptr;
}

std::vector<Mode> list() {
    return registry();
}

bool set_active(const std::string& name) {
    if (!get(name)) return false;
    std::lock_guard<std::mutex> lock(active_mutex());
    active_name() = name;
    return true;
}

std::string active() {
    std::lock_guard<std::mutex> lock(active_mutex());
    return active_name();
}

} // namespace modes
