// managents — 45° desk enclosure for the LCDWiki 4.0" ESP32-32E display (E32R40T).
//
// Two printed parts, both support-free:
//   body  — a wedge whose front face is inclined 45°; printed standing on its base.
//   bezel — the frame around the screen; printed face down.
// The board drops into the face from the front and is clamped between the two
// with 4 self-tapping screws (ST2.9 x 9.5) that cut their own thread in the
// printed corner supports: no inserts, no nuts. USB-C exits on the right side.
//
// Render one part:   openscad -D 'part="body"' -o body.stl managents_case.scad
// Besides the printed parts: "assembly" (preview) and "section" (assembly cut in
// half).

include <lib/e32r40t.scad>

part = "assembly"; // [assembly, section, body, bezel]

/* [Shape] */
face_angle = 45;    // screen inclination from the desk, degrees; only 45 prints without supports
lip_h = 8;          // height of the vertical front lip below the screen face
top_flat = 8;       // flat strip along the top edge
base_depth = 88;    // front-to-back footprint; deeper = harder to tip when touched
wall = 2.4;         // shell wall thickness
rim = 4;            // face margin around the board opening
fit = 0.4;          // clearance between board and printed parts
chamfer = 1.0;      // 45° chamfer on the outer edges (0 for none); hides elephant foot and the bezel seam

/* [Screen frame] */
glass_gap = 0.5;    // air gap above the touch panel, more than the stack's tolerance: the bezel never presses it
bezel_top = 1.6;    // frame thickness over the glass border
window_margin = 0.4; // window grows beyond the touch view area by this much

/* [Fasteners] */
screw_l = 9.5;          // length of the self-tapping screws (ST2.9 pan head; ST3.0 x 10 also fits)
screw_d = 3.4;          // clearance in the bezel
screw_head_d = 6.2;     // counterbore for a pan head (ST2.9 head is 5.5 mm)
screw_head_h = 3.2;
post_wall = 1.2;        // plastic around the screw heads in the bezel's corner posts
pilot_d = 2.5;          // pilot hole the screws cut their thread in (about 85 % of the 2.9 mm thread)
pilot_h = 6.5;          // pilot depth: the screw's thread plus 1.5 mm for its tip
boss_wall = 2.5;        // plastic around each pilot hole in the supports under the board corners
corner_block_h = 8;     // height of those supports: 1.5 mm of plastic stays below the pilot hole

/* [Cable] */
usb_cutout = [14, 10];  // side opening for the USB-C plug (along the face, along its normal)

/* [Extras] */
grille = true;          // back grille: lets the RGB LED's light out, and a future speaker's sound
speaker_mount = false;  // ring for a 28 mm speaker behind the grille; comes with the sound feature
speaker_d = 28;
foot_d = 10.5;          // recesses for the self-adhesive rubber feet
foot_h = 1.0;

/* [Hidden] */
$fn = 48;
eps = 0.01;

// --- Derived dimensions -------------------------------------------------------

face_u = board_pcb[1] + 2 * (rim + fit); // face width (along the desk)
face_v = board_pcb[0] + 2 * (rim + fit); // face height (up the slope)
bezel_h = board_front_t + glass_gap + bezel_top;
board_origin = [rim + fit, rim + fit];   // board's lower-left corner on the face
post_d = screw_head_d + 2 * post_wall;
usb_axis_n = -board_pcb_t - board_usb_size[1] / 2; // receptacle axis, in the face frame
screw_engagement = screw_l - (bezel_h - screw_head_h) - board_pcb_t; // thread inside the pilot hole
foot_inset = foot_d / 2 + 2; // feet centres from the edges, leaving 2 mm of base around each recess

c = cos(face_angle);
s = sin(face_angle);
profile = [
    [0, 0],
    [0, lip_h],
    [face_v * c, lip_h + face_v * s],
    [face_v * c + top_flat, lip_h + face_v * s],
    [base_depth, 0],
];
height = lip_h + face_v * s;

// A tap at the top of the touch area pushes along the face normal; its line of
// action meets the desk at tap_line_y. Behind the rear feet, a hard enough tap
// tips the case over them; in front of them, no tap can.
touch_top_v = board_origin[1] + board_touch_visible[0] + board_touch_visible[2];
tap_line_y = touch_top_v * c - board_front_t * s + (lip_h + touch_top_v * s + board_front_t * c) * tan(face_angle);
feet_rear_y = base_depth - foot_inset + foot_d / 2;

assert(base_depth > face_v * c + top_flat, "base_depth must reach past the top edge");
assert(post_d / 2 < board_lcd_outline[1] - board_holes[0][1], "bezel posts would press on the LCD");
assert(usb_axis_n + usb_cutout[1] / 2 < bezel_h - bezel_top, "USB notch would cut through the bezel face");
assert(screw_engagement >= 4, "screws too short: less than 4 mm of thread in the pilot holes");
assert(screw_engagement + 1 <= pilot_h, "screws too long: their tips would hit the bottom of the pilot holes");
assert(pilot_h + 1.5 <= corner_block_h, "pilot holes too deep: less than 1.5 mm of plastic below them");
if (face_angle != 45)
    echo(str("WARNING: away from 45° the face's underside (below) or the features along its normal (above) ",
             "overhang more than 45°; check supports in the slicer"));

// --- Coordinate frames --------------------------------------------------------

// Face frame: u along the desk, v up the slope, n out of the screen; n = 0 is
// the face surface, where the PCB's front copper sits.
module on_face() {
    translate([0, 0, lip_h]) rotate([face_angle, 0, 0]) children();
}

// Board frame (see lib/e32r40t.scad) inside the face frame: landscape with the
// USB-C edge on the right, which is the firmware's default rotation.
module on_board() {
    translate([board_origin[0] + board_pcb[1], board_origin[1], 0]) rotate([0, 0, 90]) children();
}

// A square of side `size` just outside the board and its clearance, at the
// corner nearest hole h (board frame): where corner features reach the walls.
module beyond_corner(h, size) {
    translate([h[0] < board_pcb[0] / 2 ? -fit - size : board_pcb[0] + fit,
               h[1] < board_pcb[1] / 2 ? -fit - size : board_pcb[1] + fit])
        square(size);
}

// --- Body ---------------------------------------------------------------------

// Cuts every corner of a convex polygon, with legs k long.
function chamfered(pts, k) = [
    for (i = [0 : len(pts) - 1])
        let (p = pts[i], a = pts[(i + len(pts) - 1) % len(pts)], b = pts[(i + 1) % len(pts)])
            each [p + k * (a - p) / norm(a - p), p + k * (b - p) / norm(b - p)]
];

// Extrudes a side profile along the desk, from u = u0.
module extrude_profile(width, u0 = 0) {
    translate([u0, 0, 0]) rotate([90, 0, 90]) linear_extrude(width) children();
}

// The outer shape, every edge chamfered, when inset = 0; the cavity otherwise.
// The profile is convex, so the hull is exact.
module wedge(inset = 0) {
    if (inset == 0 && chamfer > 0)
        hull() {
            extrude_profile(face_u - 2 * chamfer, chamfer) polygon(chamfered(profile, chamfer));
            extrude_profile(face_u) offset(delta = -chamfer) polygon(chamfered(profile, chamfer));
        }
    else
        extrude_profile(face_u - 2 * inset, inset) offset(delta = -inset) polygon(profile);
}

// The PCB sits flush with the face, on the corner blocks.
module board_opening() {
    on_face() on_board()
        translate([0, 0, -wall - 1]) linear_extrude(wall + 2) board_outline(fit);
}

// Supports under the four board corners: the board rests on them and the screws
// cut their thread in the pilot holes in them. Round towards the parts on the
// back of the PCB, square into the walls they join. They run along the face
// normal, so at 45° none of their sides overhangs more than the face itself.
module corner_blocks() {
    intersection() {
        wedge();
        on_face() on_board()
            translate([0, 0, -board_pcb_t - corner_block_h])
                linear_extrude(corner_block_h)
                    for (h = board_holes) hull() {
                        translate(h) circle(d = pilot_d + 2 * boss_wall);
                        beyond_corner(h, rim);
                    }
    }
}

module pilot_holes() {
    lead_in = 0.4; // chamfer at the mouth: guides the screw and takes the plastic its thread pushes up
    on_face() on_board()
        for (h = board_holes) translate([h[0], h[1], 0]) {
            translate([0, 0, -board_pcb_t - pilot_h]) cylinder(d = pilot_d, h = pilot_h + eps);
            translate([0, 0, -board_pcb_t - lead_in])
                cylinder(d1 = pilot_d, d2 = pilot_d + 2 * lead_in, h = lead_in + eps);
        }
}

// Side opening for the USB-C plug, centred on the receptacle (face frame). It
// reaches above the face, so the same cut notches the bezel and a thick plug
// overmold clears both parts.
module usb_cut() {
    on_board()
        translate([board_usb_center_x, 2, usb_axis_n])
            rotate([90, 0, 0])
                linear_extrude(rim + fit + wall + 4)
                    offset(r = 1.5) offset(delta = -1.5) square(usb_cutout, center = true);
}

// Back face: centre point and outward orientation, for the grille.
back_a = profile[3];
back_b = profile[4];
back_angle = atan2(back_b[0] - back_a[0], back_a[1] - back_b[1]); // from vertical

module on_back_face() {
    translate([face_u / 2, (back_a[0] + back_b[0]) / 2, (back_a[1] + back_b[1]) / 2])
        rotate([-90 + back_angle, 0, 0]) children(); // local z = outward normal
}

module grille_holes() {
    pitch = 3.2;
    on_back_face()
        for (i = [-4:4], j = [-4:4]) {
            x = i * pitch + (j % 2) * pitch / 2;
            y = j * pitch * 0.866;
            if (x * x + y * y < pow(speaker_d / 2 - 3, 2))
                translate([x, y, -wall - 1]) cylinder(d = 2, h = wall + 2, $fn = 12);
        }
}

// Its underside overhangs more than 45°: print with supports when enabled.
module speaker_ring() {
    on_back_face()
        translate([0, 0, -wall - 3])
            difference() {
                cylinder(d = speaker_d + 0.6 + 3.2, h = 3 + eps);
                translate([0, 0, -eps]) cylinder(d = speaker_d + 0.6, h = 3 + 3 * eps);
            }
}

// At the corners, so the case tips as late as possible (see tap_line_y).
module feet_recesses() {
    for (x = [foot_inset, face_u - foot_inset], y = [foot_inset, base_depth - foot_inset])
        translate([x, y, -eps]) cylinder(d = foot_d, h = foot_h + eps);
}

module body() {
    difference() {
        union() {
            difference() {
                wedge();
                wedge(inset = wall);
                board_opening();
            }
            corner_blocks();
            if (speaker_mount) intersection() { wedge(); speaker_ring(); }
        }
        pilot_holes();
        on_face() usb_cut();
        if (grille) grille_holes();
        feet_recesses();
    }
}

// --- Bezel --------------------------------------------------------------------

module bezel_window() {
    r = board_touch_visible;
    grow = window_margin;
    // 45° chamfer on the outside improves the view at an angle.
    hull() {
        translate([r[0] - grow, r[1] - grow, -1]) cube([r[2] + 2 * grow, r[3] + 2 * grow, bezel_h - bezel_top + 1]);
        translate([r[0] - grow - bezel_top, r[1] - grow - bezel_top, bezel_h])
            cube([r[2] + 2 * (grow + bezel_top), r[3] + 2 * (grow + bezel_top), eps]);
    }
}

module bezel() {
    difference() {
        union() {
            difference() {
                hull() { // chamfered front edges
                    cube([face_u, face_v, bezel_h - chamfer]);
                    translate([chamfer, chamfer, 0]) cube([face_u - 2 * chamfer, face_v - 2 * chamfer, bezel_h]);
                }
                // Room for the LCD + touch stack and the parts on the PCB's end strips.
                on_board() translate([0, 0, -1]) linear_extrude(board_front_t + glass_gap + 1) board_outline(fit);
            }
            // Corner posts press the PCB down at its mounting pads and carry the
            // screw heads; each reaches into the rim, so it is part of the frame.
            on_board()
                linear_extrude(bezel_h - eps)
                    for (h = board_holes) hull() {
                        translate(h) circle(d = post_d);
                        beyond_corner(h, post_wall);
                    }
        }
        on_board() {
            bezel_window();
            for (h = board_holes) translate([h[0], h[1], 0]) {
                translate([0, 0, -1]) cylinder(d = screw_d, h = bezel_h + 2);
                translate([0, 0, bezel_h - screw_head_h]) cylinder(d = screw_head_d, h = screw_head_h + 1);
            }
        }
        usb_cut();
    }
}

// --- Output -------------------------------------------------------------------

module assembly() {
    color("whitesmoke") body();
    on_face() {
        color("dimgray") bezel();
        on_board() {
            e32r40t_mock();
            e32r40t_screen();
        }
    }
}

if (part == "body") {
    body();
} else if (part == "bezel") {
    // Face down on the bed.
    translate([0, face_v, bezel_h]) rotate([180, 0, 0]) bezel();
} else if (part == "section") {
    // Assembly cut through the middle, to check clearances behind the board.
    intersection() {
        assembly();
        translate([face_u / 2, -1, -1]) cube([face_u, base_depth + 2, height + bezel_h + 10]);
    }
} else {
    assembly();
}

echo(str("managents case: ", face_u, " x ", base_depth, " x ", height, " mm (w x d x h)"));
echo(str("ST2.9 x ", screw_l, " screws engage ", screw_engagement, " mm of thread in the ", pilot_d, " mm pilot holes"));
echo(str("tap-proof depth: the rear feet end at y = ", feet_rear_y, "; no tap can tip the case if they reach y = ",
         tap_line_y));
