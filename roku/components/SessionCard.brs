sub init()
    m.bg = m.top.findNode("bg")
    m.name = m.top.findNode("name")
    m.activity = m.top.findNode("activity")
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
    m.activity.width = w - 56
    ' Width 0 sizes the status to its text, so placeFolder can measure it.
    m.status.width = 0
    m.status.translation = [28, h - 76]
    m.folder.height = 40
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
    m.activity.color = style.fg
    m.status.color = style.fg
    m.folder.color = style.fg

    ' With a title, it is the headline and the folder name moves down beside
    ' the status. Without one, the folder name is the headline as before.
    name = a.name
    if name = invalid then name = ""
    title = a.title
    if title = invalid then title = ""
    if title <> ""
        m.name.font = "font:SmallBoldSystemFont"
        m.name.text = title
        m.folder.text = name
    else
        m.name.font = "font:MediumBoldSystemFont"
        m.name.text = name
        m.folder.text = ""
    end if
    placeActivity(a.activity)

    age = m.top.elapsed
    if a.age <> invalid then age = age + a.age
    m.status.text = style.label + "   " + formatAge(age)
    placeFolder()

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

' The folder shares the status row, right-aligned in the space the status text
' leaves, and is shortened with an ellipsis when it does not fit.
sub placeFolder()
    gap = 24
    statusWidth = m.status.boundingRect().width
    left = 28 + statusWidth + gap
    space = m.top.cardWidth - 28 - left
    if space < 0 then space = 0
    m.folder.translation = [left, m.top.cardHeight - 72]
    m.folder.width = space
    m.folder.visible = space > 0
end sub

' The activity line sits under the headline: what Claude is doing, or what it
' needs from you. It takes the lines that fit above the status row (at most
' two), and is hidden when the helper sends none or there is no room.
sub placeActivity(activity as Dynamic)
    if activity = invalid then activity = ""
    m.activity.text = activity
    m.activity.visible = false
    if activity = "" then return
    m.activity.maxLines = 1
    lineHeight = m.activity.boundingRect().height
    top = m.name.translation[1] + m.name.boundingRect().height + 6
    bottom = m.top.cardHeight - 80 ' the status row starts at cardHeight - 76
    if lineHeight <= 0 then return
    lines = Int((bottom - top) / lineHeight)
    if lines < 1 then return
    if lines > 2 then lines = 2
    m.activity.maxLines = lines
    m.activity.translation = [28, top]
    m.activity.visible = true
end sub
