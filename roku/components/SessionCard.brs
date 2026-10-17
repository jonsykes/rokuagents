sub init()
    m.bg = m.top.findNode("bg")
    m.name = m.top.findNode("name")
    m.status = m.top.findNode("status")
    m.ctxTrack = m.top.findNode("ctxTrack")
    m.ctxFill = m.top.findNode("ctxFill")
    layout()
end sub

sub layout()
    w = m.top.cardWidth
    h = m.top.cardHeight
    m.bg.width = w
    m.bg.height = h
    m.name.width = w - 56
    m.status.width = w - 56
    m.status.translation = [28, h - 76]
    m.ctxTrack.width = w
    m.ctxTrack.height = 12
    m.ctxTrack.translation = [0, h - 12]
    m.ctxFill.height = 12
    m.ctxFill.translation = [0, h - 12]
    render()
end sub

sub render()
    a = m.top.agent
    if a = invalid or a.status = invalid then return

    style = statusStyle(a.status)
    m.bg.color = style.bg
    m.name.color = style.fg
    m.status.color = style.fg

    name = a.name
    if name = invalid then name = ""
    m.name.text = name

    age = m.top.elapsed
    if a.age <> invalid then age = age + a.age
    m.status.text = style.label + "   " + formatAge(age)

    opacity = 1.0
    if a.status = "idle" then opacity = 0.55
    if a.status = "error" and age < 60 and not m.top.blinkOn then opacity = 0.35
    m.top.opacity = opacity

    ' Context bar: only when the limit is known, never numbers.
    fillWidth = 0
    ctx = a.ctx
    hasLimit = false
    if type(ctx) = "roAssociativeArray"
        if ctx.limit <> invalid and ctx.used <> invalid
            if ctx.limit > 0 then hasLimit = true
        end if
    end if
    if hasLimit
        fraction = ctx.used / ctx.limit
        if fraction > 1 then fraction = 1
        fillWidth = Int(m.top.cardWidth * fraction)
    end if
    m.ctxTrack.visible = hasLimit
    m.ctxFill.visible = fillWidth > 0
    m.ctxFill.width = fillWidth
end sub
