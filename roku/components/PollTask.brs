sub init()
    m.top.functionName = "pollForever"
end sub

sub pollForever()
    stateUrl = m.top.url + "/state"
    while true
        fetchOnce(stateUrl)
        sleep(m.top.intervalMs)
    end while
end sub

sub fetchOnce(stateUrl as String)
    ' A fresh port per request, so a late event from a cancelled request can
    ' never be mistaken for the answer to this one.
    port = CreateObject("roMessagePort")
    xfer = CreateObject("roUrlTransfer")
    xfer.SetMessagePort(port)
    xfer.SetUrl(stateUrl)
    xfer.RetainBodyOnError(true)
    if m.top.token <> "" then xfer.AddHeader("Authorization", "Bearer " + m.top.token)

    if not xfer.AsyncGetToString()
        m.top.failure = "Could not start a request to " + stateUrl
        return
    end if
    msg = wait(3000, port)
    if msg = invalid
        xfer.AsyncCancel()
        m.top.failure = "No answer from " + stateUrl
        return
    end if
    if type(msg) <> "roUrlEvent" then return

    code = msg.GetResponseCode()
    if code = 200
        frame = ParseJson(msg.GetString())
        if isStateFrame(frame)
            m.top.failure = ""
            m.top.frame = frame
        else
            m.top.failure = "The helper sent something this channel cannot read"
        end if
    else if code = 401
        m.top.failure = "The helper rejected the token. Rebuild the channel with the right one."
    else if code = 503
        m.top.failure = "The helper is starting..."
    else if code < 0
        m.top.failure = "Cannot reach " + m.top.url + " (" + msg.GetFailureReason() + ")"
    else
        m.top.failure = "Unexpected answer " + code.ToStr() + " from " + stateUrl
    end if
end sub

function isStateFrame(frame as Dynamic) as Boolean
    if type(frame) <> "roAssociativeArray" then return false
    if frame.v = invalid or frame.t = invalid then return false
    if frame.v <> 1 or frame.t <> "state" then return false
    return type(frame.agents) = "roArray"
end function
