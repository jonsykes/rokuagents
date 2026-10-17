' Shared formatting helpers, included by the scene and the card.

' Colors and label for each protocol status. fg is the text color on bg.
function statusStyle(status as Dynamic) as Object
    if status = invalid then return { bg: "0x37474FFF", fg: "0xFFFFFFFF", label: "Unknown" }
    if status = "working" then return { bg: "0x2E7D32FF", fg: "0xFFFFFFFF", label: "Working" }
    if status = "waiting" then return { bg: "0xF9A825FF", fg: "0x1A1A1AFF", label: "Needs you" }
    if status = "error" then return { bg: "0xC62828FF", fg: "0xFFFFFFFF", label: "Error" }
    if status = "idle" then return { bg: "0x263238FF", fg: "0xB0BEC5FF", label: "Idle" }
    return { bg: "0x37474FFF", fg: "0xFFFFFFFF", label: "Unknown" }
end function

' 42s, 5m, 1h 12m, 2h, 1d 2h, 3d: the same shapes as the desk display.
function formatAge(seconds as Dynamic) as String
    s = Int(seconds)
    if s < 0 then s = 0
    if s < 60 then return s.ToStr() + "s"
    if s < 3600 then return (s \ 60).ToStr() + "m"
    if s < 86400
        h = s \ 3600
        mins = (s mod 3600) \ 60
        if mins = 0 or h >= 10 then return h.ToStr() + "h"
        return h.ToStr() + "h " + mins.ToStr() + "m"
    end if
    d = s \ 86400
    h = (s mod 86400) \ 3600
    if h = 0 or d >= 3 then return d.ToStr() + "d"
    return d.ToStr() + "d " + h.ToStr() + "h"
end function

' HH:MM from a local UNIX time (host time plus its UTC offset).
function formatClock(localEpoch as Dynamic) as String
    daySecs = localEpoch mod 86400
    if daySecs < 0 then daySecs = daySecs + 86400
    return pad2(daySecs \ 3600) + ":" + pad2((daySecs mod 3600) \ 60)
end function

function pad2(n as Dynamic) as String
    s = n.ToStr()
    if Len(s) < 2 then s = "0" + s
    return s
end function
