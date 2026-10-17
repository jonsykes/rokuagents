' Entry points. Main runs the channel; RunScreenSaver runs the same screen as a
' Roku screensaver (Settings > Theme > Screensavers once the channel is sideloaded).

sub Main(args as Dynamic)
    showSessions(false)
end sub

sub RunScreenSaver()
    showSessions(true)
end sub

sub showSessions(asScreensaver as Boolean)
    screen = CreateObject("roSGScreen")
    port = CreateObject("roMessagePort")
    screen.SetMessagePort(port)
    scene = screen.CreateScene("SessionsScene")
    screen.Show()
    scene.screensaver = asScreensaver
    while true
        msg = wait(0, port)
        if type(msg) = "roSGScreenEvent" and msg.IsScreenClosed() then return
    end while
end sub
