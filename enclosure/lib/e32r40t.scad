// LCDWiki 4.0" ESP32-32E display board (E32R40T / E32N40T) — mechanical data.
//
// Source: vendor outline drawing E32R40T_Size.pdf (LCM OUTLINE V1.0, 2025-04-15),
// see docs/hardware.md. Tolerance ±0.2 mm unless noted.
//
// Board frame, looking at the screen in portrait with the USB-C edge at the
// bottom: x to the right (0..60.88), y up (0..111.11, USB-C at y = 0, the
// ESP32 antenna at y = 111.11), z out of the screen with z = 0 on the PCB's
// front copper. The PCB spans z = -1.6..0, the LCD + touch stack z = 0..4.05,
// components on the back go down to z = -1.6 - 5.09.

board_pcb = [60.88, 111.11];
board_pcb_t = 1.6;
board_corner_r = 3.5;

// Mounting holes: 4 x Ø3.2, pad Ø5.6, centres 53.28 x 104.11 apart.
board_hole_d = 3.2;
board_hole_pad_d = 5.6;
board_holes = [[3.80, 3.50], [57.08, 3.50], [3.80, 107.61], [57.08, 107.61]];

// Front stack: 0.5 tape + 2.5 LCD + 1.05 resistive touch panel.
board_front_t = 4.05;
// Rectangles as [x, y, width, height] in the board frame.
board_lcd_outline = [0.00, 8.27, 60.88, 94.57];  // LCD backlight outline
board_touch_outline = [0.20, 8.77, 60.48, 93.87]; // RTP outer dimension
board_touch_visible = [2.00, 15.62, 56.88, 85.22]; // RTP view area
board_lcd_active = [2.60, 16.67, 55.68, 83.52];  // LCD active area (pixels)

// Back side. The front shows only the LCD and the USB-C shell's solder tabs.
board_back_clearance = 5.09; // tallest SMD part below the PCB
board_antenna_notch = [19.74, 104.41, 20.00, 6.70]; // PCB cut-out under the ESP32 antenna

// USB-C receptacle on the back, centred on the y = 0 edge (standard 8.94 x 3.26 body).
board_usb_center_x = 30.44;
board_usb_size = [8.94, 3.26];

// 1.25 mm JST sockets on the back long edges (centre y), side entry. x = 60.88 edge:
board_jst_right = [["I2C", 93.10], ["SPI", 70.10], ["SPEAKER", 48.17], ["IO35/IO39", 29.44]];
// x = 0 edge:
board_jst_left = [["BAT", 25.48], ["UART", 44.92]];
board_sd_center_y = 67.50; // microSD slot, x = 0 edge

// RGB LED centre on the back, facing away from the screen (measured on the vendor
// render, ±1 mm): a case hides it, only the light that escapes is visible.
board_led = [30.8, 52.5];
// Push buttons on the back, pressed from the back (drawing: 15.38 from each long
// edge, 3.26 from the USB-C edge). Never needed: the USB bridge resets the board
// and enters the download mode on its own.
board_buttons = [["BOOT", [15.38, 3.26]], ["RESET", [45.50, 3.26]]];

module board_rect(r, z0, height) {
    translate([r[0], r[1], z0]) cube([r[2], r[3], height]);
}

// PCB outline with its rounded corners, grown by `grow` on every side.
module board_outline(grow = 0) {
    offset(r = grow) offset(r = board_corner_r) offset(delta = -board_corner_r) square(board_pcb);
}

// Simplified board for assembly previews and clearance checks.
module e32r40t_mock() {
    color("darkorange") difference() {
        translate([0, 0, -board_pcb_t]) linear_extrude(board_pcb_t) board_outline();
        for (h = board_holes) translate([h[0], h[1], -5]) cylinder(d = board_hole_d, h = 10, $fn = 24);
    }
    color("dimgray") board_rect(board_lcd_outline, 0, board_front_t - 1.05);
    color("lightblue", 0.6) board_rect(board_touch_outline, board_front_t - 1.05, 1.05);
    color("black") board_rect(board_lcd_active, board_front_t, 0.01);
    // Back-side envelope and the USB-C receptacle.
    color("gray", 0.35)
        translate([3, 8, -board_pcb_t - board_back_clearance])
            cube([board_pcb[0] - 6, board_pcb[1] - 16, board_back_clearance]);
    color("silver")
        translate([board_usb_center_x - board_usb_size[0] / 2, -0.5, -board_pcb_t - board_usb_size[1]])
            cube([board_usb_size[0], 7.35, board_usb_size[1]]);
    // Markers (not to size) for the LED and the buttons.
    color("red") translate([board_led[0] - 1, board_led[1] - 1, -board_pcb_t - 0.6]) cube([2, 2, 0.6]);
    color("white")
        for (b = board_buttons) translate([b[1][0] - 2, b[1][1] - 1.5, -board_pcb_t - 1.5]) cube([4, 3, 1.5]);
}

// A page of nine status cards on the active area, for renders. Laid out like the
// firmware's 480 x 320 landscape grid (USB-C on the right): a header strip, then
// 3 x 3 cards, working first. Colours from firmware/src/ui/theme.hpp.
module e32r40t_screen() {
    working = "#15803D";
    waiting = "#FACC15";
    error = "#DC2626";
    idle = "#27272A";
    cards = [working, working, working, waiting, error, waiting, idle, waiting, idle];

    a = board_lcd_active;
    px = a[3] / 480; // mm per pixel
    header = 28;     // the layout in pixels
    margin = 6;
    gap = 6;
    radius = 10;
    w = (480 - 2 * margin - 2 * gap) / 3;
    h = (320 - header - margin - 2 * gap) / 3;

    color("#09090B") board_rect(a, board_front_t, 0.02);
    for (i = [0 : len(cards) - 1]) {
        x = margin + (i % 3) * (w + gap); // landscape pixels: x right, y down
        y = header + floor(i / 3) * (h + gap);
        color(cards[i])
            translate([a[0] + a[2] - (y + h) * px, a[1] + a[3] - (x + w) * px, board_front_t + 0.02])
                linear_extrude(0.02)
                    offset(r = radius * px) offset(delta = -radius * px) square([h * px, w * px]);
    }
}
