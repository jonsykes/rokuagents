sub init()
    m.top.functionName = "pollForever"
end sub

' Polls {base url}/state for as long as the channel runs. The base url starts
' as the address this channel last discovered for its config (or config.json's
' url), and changes when discovery finds the helper somewhere else.
sub pollForever()
    m.configUrl = m.top.url
    m.tokenId = tokenId(m.top.token)
    m.registryKey = m.configUrl + " " + m.tokenId
    m.baseUrl = savedUrl()
    m.top.serverUrl = m.baseUrl
    m.sinceSuccess = CreateObject("roTimespan")
    m.sinceDiscovery = invalid
    while true
        if fetchOnce(m.baseUrl + "/state")
            m.sinceSuccess.Mark()
        else if shouldDiscover()
            rediscover()
        end if
        sleep(m.top.intervalMs)
    end while
end sub

function fetchOnce(stateUrl as String) as Boolean
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
        return false
    end if
    msg = wait(3000, port)
    if msg = invalid
        xfer.AsyncCancel()
        m.top.failure = "No answer from " + stateUrl
        return false
    end if
    if type(msg) <> "roUrlEvent" then return false

    code = msg.GetResponseCode()
    if code = 200
        frame = ParseJson(msg.GetString())
        if isStateFrame(frame)
            m.top.failure = ""
            m.top.frame = frame
            return true
        end if
        m.top.failure = "The helper sent something this channel cannot read"
    else if code = 401
        m.top.failure = "The helper rejected the token. Rebuild the channel with the right one."
    else if code = 503
        m.top.failure = "The helper is starting..."
    else if code < 0
        m.top.failure = "Cannot reach " + m.baseUrl + " (" + msg.GetFailureReason() + ")"
    else
        m.top.failure = "Unexpected answer " + code.ToStr() + " from " + stateUrl
    end if
    return false
end function

function isStateFrame(frame as Dynamic) as Boolean
    if type(frame) <> "roAssociativeArray" then return false
    if frame.v = invalid or frame.t = invalid then return false
    if frame.v <> 1 or frame.t <> "state" then return false
    return type(frame.agents) = "roArray"
end function

' --- discovery (docs/http-api.md#discovery) ---------------------------------

' Discovery runs after 3 seconds without a good answer, at most every 5
' seconds, and only with a token of 32 characters or more: the token is what
' proves a reply genuine, and a short one could be guessed from its id in the
' broadcast. The helper applies the same rule.
function shouldDiscover() as Boolean
    if Len(m.top.token) < 32 then return false
    if m.sinceSuccess.TotalMilliseconds() < 3000 then return false
    return m.sinceDiscovery = invalid or m.sinceDiscovery.TotalMilliseconds() >= 5000
end function

sub rediscover()
    m.sinceDiscovery = CreateObject("roTimespan")
    found = discover()
    if found = ""
        ' Nobody answered. Maybe a helper without discovery is back at the
        ' configured address, so poll that until discovery finds something.
        found = m.configUrl
    else if found <> m.savedUrl
        saveUrl(found)
    end if
    m.baseUrl = found
    m.top.serverUrl = found
end sub

' Broadcasts a discovery request to the configured port and returns the base
' url of the first reply signed with this channel's token, or "".
function discover() as String
    sock = CreateObject("roDatagramSocket")
    if sock = invalid then return ""
    port = CreateObject("roMessagePort")
    sock.SetMessagePort(port)
    bind = CreateObject("roSocketAddress")
    bind.SetPort(0)
    sock.SetAddress(bind)
    sock.SetBroadcast(true)
    sock.NotifyReadable(true)

    nonce = LCase(CreateObject("roDeviceInfo").GetRandomUUID().Replace("-", ""))
    request = "rokuagents-discover v1 " + m.tokenId + " " + nonce
    for each host in broadcastHosts()
        dest = CreateObject("roSocketAddress")
        dest.SetHostName(host)
        dest.SetPort(urlPort(m.configUrl))
        sock.SetSendToAddress(dest)
        sock.SendStr(request)
    end for

    found = ""
    elapsed = CreateObject("roTimespan")
    while found = ""
        remaining = 1500 - elapsed.TotalMilliseconds()
        if remaining <= 0 then exit while
        msg = wait(remaining, port)
        if msg = invalid then exit while
        if type(msg) = "roSocketEvent" and sock.IsReadable()
            found = verifiedUrl(sock.ReceiveStr(512), nonce)
        end if
    end while
    sock.Close()
    return found
end function

' The limited broadcast address, plus the /24 broadcast of each of the Roku's
' own addresses for routers that drop the limited one.
function broadcastHosts() as Object
    hosts = ["255.255.255.255"]
    addrs = CreateObject("roDeviceInfo").GetIPAddrs()
    for each name in addrs
        parts = addrs[name].Split(".")
        if parts.Count() = 4
            hosts.Push(parts[0] + "." + parts[1] + "." + parts[2] + ".255")
        end if
    end for
    return hosts
end function

' Returns the url in a reply "rokuagents v1 <url> <mac>" whose mac proves the
' sender holds the token and saw this nonce, or "".
function verifiedUrl(reply as String, nonce as String) as String
    parts = reply.Trim().Split(" ")
    if parts.Count() <> 4 then return ""
    if parts[0] <> "rokuagents" or parts[1] <> "v1" then return ""
    url = parts[2]
    if Left(url, 7) <> "http://" then return ""
    if LCase(parts[3]) <> sign(nonce + " " + url) then return ""
    return url
end function

' First 32 hex characters of HMAC-SHA256(token, message).
function sign(message as String) as String
    key = CreateObject("roByteArray")
    key.FromAsciiString(m.top.token)
    hmac = CreateObject("roHMAC")
    if hmac.Setup("sha256", key) <> 0 then return "-"
    data = CreateObject("roByteArray")
    data.FromAsciiString(message)
    return Left(LCase(hmac.Process(data).ToHexString()), 32)
end function

' First 16 hex characters of SHA-256(token): names the token without revealing it.
function tokenId(token as String) as String
    data = CreateObject("roByteArray")
    data.FromAsciiString(token)
    digest = CreateObject("roEVPDigest")
    digest.Setup("sha256")
    return Left(LCase(digest.Process(data)), 16)
end function

' The port of an http:// url, 80 when it has none.
function urlPort(url as String) as Integer
    hostPort = Mid(url, 8).Split("/")[0]
    colon = Instr(1, hostPort, ":")
    if colon = 0 then return 80
    return Val(Mid(hostPort, colon + 1), 10)
end function

' --- last discovered address -------------------------------------------------

' The address discovery last found for this config.json url and token. A new
' build with another url or token starts from its own config again.
function savedUrl() as String
    m.savedUrl = ""
    section = CreateObject("roRegistrySection", "rokuagents")
    if section.Exists("server")
        saved = ParseJson(section.Read("server"))
        if type(saved) = "roAssociativeArray" and isString(saved.key) and isString(saved.url)
            if saved.key = m.registryKey and Left(saved.url, 7) = "http://" then m.savedUrl = saved.url
        end if
    end if
    if m.savedUrl <> "" then return m.savedUrl
    return m.configUrl
end function

sub saveUrl(url as String)
    section = CreateObject("roRegistrySection", "rokuagents")
    section.Write("server", FormatJson({ key: m.registryKey, url: url }))
    section.Flush()
    m.savedUrl = url
end sub

function isString(value as Dynamic) as Boolean
    return GetInterface(value, "ifString") <> invalid
end function
