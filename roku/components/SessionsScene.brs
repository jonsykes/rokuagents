sub init()
    m.root = m.top.findNode("root")
    m.grid = m.top.findNode("grid")
    m.summary = m.top.findNode("summary")
    m.clock = m.top.findNode("clock")
    m.pageInfo = m.top.findNode("pageInfo")
    m.message = m.top.findNode("message")

    ' 3 x 3 cards inside the 1920 x 1080 title-safe area.
    m.columns = 3
    m.perPage = 9
    m.gap = 24
    m.cardW = 560
    m.cardH = 254

    m.cards = []
    m.page = 0
    m.frame = invalid
    m.blinkOn = true
    m.isScreensaver = false
    m.frameAge = CreateObject("roTimespan")   ' since the last frame arrived
    m.sinceKey = CreateObject("roTimespan")   ' since the last remote press
    m.sincePageTurn = CreateObject("roTimespan")
    m.sinceDrift = CreateObject("roTimespan")

    m.top.setFocus(true)

    config = loadConfig()
    if config = invalid
        showMessage("config.json is missing or has no url. Rebuild the channel with build.sh.")
        signalLaunched()
        return
    end if

    m.poller = CreateObject("roSGNode", "PollTask")
    m.poller.url = config.url
    m.poller.token = config.token
    m.poller.observeField("frame", "onFrame")
    m.poller.observeField("failure", "onFailure")
    m.poller.control = "RUN"

    m.tick = startTimer(1.0, "onTick")
    m.blink = startTimer(0.5, "onBlink")
    showMessage("Connecting to your computer...")
    signalLaunched()
end sub

' Tells the Roku the first screen is up. Roku expects every channel to send
' this; without it a screensaver launched by "InstantOn" can stay pended,
' never shown, which is how the screensaver went blank.
sub signalLaunched()
    m.top.signalBeacon("AppLaunchComplete")
end sub

function loadConfig() as Dynamic
    raw = ReadAsciiFile("pkg:/config.json")
    if raw = "" then return invalid
    cfg = ParseJson(raw)
    if type(cfg) <> "roAssociativeArray" then return invalid
    if cfg.url = invalid or cfg.url = "" then return invalid
    if cfg.token = invalid then cfg.token = ""
    return cfg
end function

function startTimer(seconds as Float, callback as String) as Object
    timer = CreateObject("roSGNode", "Timer")
    timer.duration = seconds
    timer.repeat = true
    timer.observeField("fire", callback)
    m.top.appendChild(timer)
    timer.control = "start"
    return timer
end function

sub onScreensaverChanged()
    m.isScreensaver = m.top.screensaver
end sub

' --- data in ---------------------------------------------------------------

sub onFrame()
    m.frame = m.poller.frame
    m.frameAge.Mark()
    render()
end sub

sub onFailure()
    ' Before the first frame, say why we are not connected yet. After that,
    ' onTick decides when a failure has lasted long enough to show.
    if m.frame = invalid and m.poller.failure <> ""
        showMessage(m.poller.failure)
    end if
end sub

' --- clocks ----------------------------------------------------------------

sub onTick()
    if m.frame = invalid then return

    if m.frameAge.TotalSeconds() >= 6
        reason = m.poller.failure
        if reason = "" then reason = "Is your computer asleep?"
        m.grid.opacity = 0.2
        showMessage("Reconnecting... " + reason)
        return
    end if

    if m.isScreensaver
        if m.sincePageTurn.TotalSeconds() >= 10 then changePage(1)
        if m.sinceDrift.TotalSeconds() >= 60 then drift()
    else if m.page > 0 and m.sinceKey.TotalSeconds() >= 30
        m.page = 0
    end if
    render()
end sub

sub onBlink()
    m.blinkOn = not m.blinkOn
    for each card in m.cards
        card.blinkOn = m.blinkOn
    end for
end sub

sub drift()
    m.sinceDrift.Mark()
    m.root.translation = [Rnd(41) - 21, Rnd(41) - 21]
end sub

' --- drawing ---------------------------------------------------------------

sub render()
    if m.frame = invalid then return
    agents = m.frame.agents
    total = agents.Count()
    pages = pageCount(total)
    if m.page >= pages then m.page = pages - 1
    elapsed = m.frameAge.TotalSeconds()

    m.grid.opacity = 1.0
    if total = 0
        showMessage("No Claude sessions open")
    else
        hideMessage()
    end if

    first = m.page * m.perPage
    count = total - first
    if count > m.perPage then count = m.perPage
    if count < 0 then count = 0
    ensureCards(count)
    for i = 0 to count - 1
        card = m.cards[i]
        card.agent = agents[first + i]
        card.elapsed = elapsed
        card.blinkOn = m.blinkOn
    end for

    renderHeader(agents, pages, elapsed)
end sub

sub ensureCards(count as Integer)
    while m.cards.Count() < count
        i = m.cards.Count()
        card = CreateObject("roSGNode", "SessionCard")
        card.cardWidth = m.cardW
        card.cardHeight = m.cardH
        col = i mod m.columns
        row = i \ m.columns
        card.translation = [col * (m.cardW + m.gap), row * (m.cardH + m.gap)]
        m.grid.appendChild(card)
        m.cards.Push(card)
    end while
    while m.cards.Count() > count
        m.grid.removeChild(m.cards.Pop())
    end while
end sub

sub renderHeader(agents as Object, pages as Integer, elapsed as Integer)
    working = 0
    waiting = 0
    errors = 0
    for each a in agents
        if a.status = "working" then working = working + 1
        if a.status = "waiting" then waiting = waiting + 1
        if a.status = "error" then errors = errors + 1
    end for
    parts = []
    if working > 0 then parts.Push(working.ToStr() + " working")
    if waiting > 0 then parts.Push(waiting.ToStr() + " need you")
    if errors > 0 then parts.Push(errors.ToStr() + " failed")
    if parts.Count() = 0 and agents.Count() > 0 then parts.Push("All idle")
    m.summary.text = parts.Join(",  ")

    tz = 0
    if m.frame.tz <> invalid then tz = m.frame.tz
    m.clock.text = formatClock(m.frame.now + tz + elapsed)

    info = ""
    if pages > 1 then info = "Page " + (m.page + 1).ToStr() + " of " + pages.ToStr()
    more = 0
    if m.frame.more <> invalid then more = m.frame.more
    if more > 0
        if info <> "" then info = info + "   "
        info = info + "+" + more.ToStr() + " more"
    end if
    m.pageInfo.text = info
end sub

sub showMessage(text as String)
    m.message.text = text
    m.message.visible = true
end sub

sub hideMessage()
    m.message.visible = false
end sub

' --- remote ----------------------------------------------------------------

function pageCount(total as Integer) as Integer
    pages = (total + m.perPage - 1) \ m.perPage
    if pages < 1 then pages = 1
    return pages
end function

sub changePage(delta as Integer)
    m.sincePageTurn.Mark()
    if m.frame = invalid then return
    pages = pageCount(m.frame.agents.Count())
    m.page = (m.page + delta + pages) mod pages
    render()
end sub

function onKeyEvent(key as String, press as Boolean) as Boolean
    if not press then return false
    m.sinceKey.Mark()
    if key = "right" or key = "down" or key = "fastforward"
        changePage(1)
        return true
    end if
    if key = "left" or key = "up" or key = "rewind"
        changePage(-1)
        return true
    end if
    return false
end function
