sub init()
    m.bg = m.top.findNode("bg")
    m.name = m.top.findNode("name")
    m.status = m.top.findNode("status")
    m.folder = m.top.findNode("folder")
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
    ' The folder shares the status row, right-aligned, when a title is shown.
    m.folder.width = w - 56 - 220
    m.folder.height = 40
    m.folder.translation = [28 + 220, h - 72]
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
    m.folder.color = style.fg

    ' With a title, it is the headline and the folder name moves down beside
    ' the status. Without one, the folder name is the headline as before.
    name = a.name
    if name = invalid then name = ""
    title = a.title
    if title = invalid then title = ""
    if title <> ""
        m.name.font = "font:MediumBoldSystemFont"
        m.name.maxLines = 3
        m.name.text = title
        m.folder.text = name
    else
        m.name.font = "font:LargeBoldSystemFont"
        m.name.maxLines = 2
        m.name.text = name
        m.folder.text = ""
    end if

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
