#include "managents/core/application.hpp"

namespace managents::core {
namespace {

/// True if an agent shows an error young enough to blink.
bool anyBlinks(const HostState& host, std::uint32_t elapsedSeconds) {
    for (std::uint8_t i = 0; i < host.agentCount; ++i) {
        if (blinks(host.agents[i], elapsedSeconds)) {
            return true;
        }
    }
    return false;
}

}  // namespace

bool dimsBacklight(SceneKind kind, Attention attention, std::uint32_t msUnchanged) {
    switch (kind) {
        case SceneKind::Reconnecting:
            return msUnchanged >= kDimWhileAsleepAfterMs;
        case SceneKind::NoAgents:
        case SceneKind::Agents:
            return attention == Attention::Quiet && msUnchanged >= kDimWhileQuietAfterMs;
        case SceneKind::Connecting:
        case SceneKind::SetupNeeded:
            break;
    }
    return false;
}

Application::Application(Display& display, StatusIndicator& indicator, HostLink& link, const DeviceInfo& device,
                         const ScreenGeometry& geometry, Pager pager)
    : display_(display), indicator_(indicator), link_(link), device_(device), geometry_(geometry), pager_(pager) {}

void Application::begin(std::uint32_t nowMs) {
    lastContactMs_ = nowMs;  // the setup hint counts from boot
    sendHello();
    tick(nowMs);
}

void Application::onReceive(const char* data, std::size_t length, std::uint32_t nowMs) {
    for (std::size_t i = 0; i < length; ++i) {
        if (assembler_.push(data[i])) {
            handleLine(assembler_.line(), assembler_.length(), nowMs);
        }
    }
}

void Application::onTouch(bool pressed, std::uint32_t nowMs) {
    if (!tap_.update(pressed, nowMs)) {
        return;
    }
    restingSinceMs_ = nowMs;
    if (dimmed_) {
        dimmed_ = false;  // the tap only wakes the screen
        return;
    }
    if (hostConnected(nowMs)) {
        pager_.next(host_.agentCount, nowMs);
    }
}

void Application::tick(std::uint32_t nowMs) {
    linkFresh_ = hostConnected(nowMs);
    setupHintDue_ = setupHintDue_ || nowMs - lastContactMs_ >= kSetupHintAfterMs;
    const std::uint32_t msSinceFrame = nowMs - lastFrameMs_;
    const bool blinkOn = (nowMs / kBlinkHalfPeriodMs) % 2 == 0;
    pager_.update(linkFresh_ ? host_.agentCount : 0, nowMs);

    SceneInput input;
    input.host = linkFresh_ ? &host_ : nullptr;
    input.msSinceFrame = msSinceFrame;
    input.blinkOn = blinkOn;
    input.page = pager_.page();
    input.link = linkState();
    const Scene scene = buildScene(input, geometry_);
    if (!sceneShown_ || scene != shownScene_) {
        display_.present(scene);
        shownScene_ = scene;
        sceneShown_ = true;
    }

    const Attention attention = linkFresh_ ? summarize(host_) : Attention::Disconnected;
    updateBacklight(scene.kind, attention, nowMs);
    const bool freshError = linkFresh_ && anyBlinks(host_, msSinceFrame / 1000);
    indicator_.show(attention, blinkOn || !freshError);
}

bool Application::hostConnected(std::uint32_t nowMs) const {
    return linkFresh_ && nowMs - lastFrameMs_ < kLinkTimeoutMs;
}

LinkState Application::linkState() const {
    if (hasFrame_) {
        return LinkState::Lost;
    }
    return setupHintDue_ ? LinkState::SetupNeeded : LinkState::Connecting;
}

void Application::updateBacklight(SceneKind kind, Attention attention, std::uint32_t nowMs) {
    if (kind != restingKind_ || attention != restingAttention_) {
        restingKind_ = kind;
        restingAttention_ = attention;
        restingSinceMs_ = nowMs;
        dimmed_ = false;
    }
    // Latched, so that it stays dim across the millis() wrap.
    dimmed_ = dimmed_ || dimsBacklight(kind, attention, nowMs - restingSinceMs_);
    display_.setBrightness(dimmed_ ? Brightness::Dimmed : Brightness::Full);
}

void Application::handleLine(const char* line, std::size_t length, std::uint32_t nowMs) {
    switch (decodeHostMessage(line, length, host_)) {
        case MessageType::State:
            hasFrame_ = true;
            linkFresh_ = true;
            lastFrameMs_ = nowMs;
            lastContactMs_ = nowMs;
            setupHintDue_ = false;
            break;
        case MessageType::Hello:
            lastContactMs_ = nowMs;  // a helper probing for displays: it is there
            setupHintDue_ = false;
            sendHello();
            break;
        case MessageType::Invalid:
        case MessageType::Ignored:
            break;
    }
}

void Application::sendHello() {
    char line[192];
    if (encodeHello(device_, line, sizeof line) > 0) {
        link_.sendLine(line);
    }
}

}  // namespace managents::core
