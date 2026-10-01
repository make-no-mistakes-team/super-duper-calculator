#!/usr/bin/env python3
"""Draw the 22 original 64px achievement sprites and export LibreSprite sources.

Run with Pillow installed (the local pixel-art-python workflow is supported):
    pixel-art-python art/achievements/generate.py --contact-sheet /tmp/achievement-art.png
All drawing is on the native pixel grid; there is no resampling or antialiasing.
LibreSprite imports the completed RGBA artwork into genuinely editable ASE files.
"""

from __future__ import annotations

import argparse
from pathlib import Path
import shutil
import subprocess

from PIL import Image, ImageDraw, ImageFont


PALETTE = {
    "ink": "#11191f",
    "shadow": "#172831",
    "steel": "#2d4651",
    "steel_mid": "#486772",
    "steel_light": "#86a6aa",
    "paper": "#d3dfcf",
    "white": "#f4f3d5",
    "cyan_dark": "#235963",
    "cyan": "#54b9bc",
    "cyan_light": "#a8ece1",
    "lime_dark": "#4d713e",
    "lime": "#a8cf5b",
    "lime_light": "#e0ee92",
    "amber_dark": "#77512f",
    "amber": "#d99847",
    "amber_light": "#f4ce82",
}

DIGITS = {
    "2": ("1111", "0001", "0001", "1111", "1000", "1000", "1111"),
    "4": ("1001", "1001", "1001", "1111", "0001", "0001", "0001"),
    "6": ("1111", "1000", "1000", "1111", "1001", "1001", "1111"),
    "7": ("1111", "0001", "0010", "0010", "0100", "0100", "0100"),
}


class Sprite:
    def __init__(self):
        self.image = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
        self.draw = ImageDraw.Draw(self.image)

    def rect(self, box, color):
        self.draw.rectangle(box, fill=PALETTE.get(color, color))

    def poly(self, points, color):
        self.draw.polygon(points, fill=PALETTE.get(color, color))

    def line(self, points, color, width=1):
        self.draw.line(points, fill=PALETTE.get(color, color), width=width)

    def ellipse(self, box, color):
        self.draw.ellipse(box, fill=PALETTE.get(color, color))

    def star(self, x, y, color="cyan_light", size=2):
        self.rect((x, y-size, x, y+size), color)
        self.rect((x-size, y, x+size, y), color)
        self.rect((x, y, x, y), "white")

    def numeral(self, text, x, y, color, scale=1, gap=1):
        for char in text:
            for row, cells in enumerate(DIGITS[char]):
                for col, cell in enumerate(cells):
                    if cell == "1":
                        self.rect((x+col*scale, y+row*scale,
                                   x+(col+1)*scale-1, y+(row+1)*scale-1), color)
            x += 4*scale + gap


def answer_found():
    s = Sprite()
    # A navigation compass suspended in an orbital cradle.
    s.line([(6, 37), (8, 29), (17, 17), (31, 10), (46, 10), (55, 15)], "cyan_dark")
    s.line([(9, 45), (18, 52), (35, 55), (51, 49), (58, 39)], "cyan")
    s.rect((16, 48, 19, 49), "cyan_light")
    s.star(8, 17, "cyan_light")
    s.star(53, 7, "lime_light")
    s.rect((53, 47, 54, 48), "amber_light")
    s.rect((6, 49, 7, 50), "cyan")
    s.rect((30, 4, 34, 11), "ink")
    s.rect((31, 5, 33, 9), "amber")
    s.rect((31, 5, 32, 7), "amber_light")
    s.ellipse((10, 11, 54, 55), "ink")
    s.ellipse((12, 12, 52, 53), "amber_dark")
    s.ellipse((12, 12, 50, 50), "amber")
    s.ellipse((14, 13, 48, 48), "amber_light")
    s.ellipse((17, 16, 48, 49), "ink")
    s.ellipse((19, 18, 46, 47), "cyan_dark")
    s.ellipse((20, 18, 44, 44), "shadow")
    s.line([(22, 22), (25, 20), (38, 20), (42, 24)], "steel_mid")
    for x, y in ((31, 17), (46, 31), (31, 47), (17, 31)):
        s.rect((x, y, x+2, y+2), "amber_light")
    # The bright needle aims beyond the compass, toward a cosmic answer.
    s.poly([(32, 20), (39, 33), (32, 39), (25, 33)], "ink")
    s.poly([(32, 21), (32, 35), (26, 33)], "lime_light")
    s.poly([(32, 21), (38, 33), (32, 35)], "lime")
    s.poly([(26, 34), (32, 35), (32, 44)], "cyan")
    s.poly([(32, 35), (38, 34), (32, 44)], "cyan_dark")
    s.ellipse((30, 31, 34, 35), "white")
    s.rect((37, 39, 49, 51), "ink")
    s.rect((38, 40, 48, 50), "steel")
    s.numeral("42", 39, 41, "lime_light")
    # Keep the authored needle's single-pixel notch across Pillow versions.
    s.rect((27, 34, 27, 34), "ink")
    return s.image


def six_seven():
    s = Sprite()
    # A chunky broadcast tabloid: aerials, lamps, metal ribs and glowing 67.
    s.line([(20, 14), (14, 6)], "ink", 3)
    s.line([(20, 13), (14, 6)], "steel_light")
    s.line([(39, 13), (46, 5)], "ink", 3)
    s.line([(39, 12), (46, 5)], "steel_light")
    s.rect((13, 5, 15, 7), "cyan_light")
    s.rect((45, 4, 47, 6), "amber_light")
    s.rect((26, 44, 36, 57), "ink")
    s.rect((28, 45, 31, 55), "steel_mid")
    s.rect((32, 45, 34, 55), "steel")
    s.poly([(21, 55), (41, 55), (45, 59), (17, 59)], "ink")
    s.rect((22, 56, 40, 57), "steel_light")
    s.rect((18, 58, 44, 59), "steel")
    s.poly([(9, 12), (52, 12), (56, 16), (56, 43), (52, 47), (9, 47), (6, 43), (6, 16)], "ink")
    s.rect((9, 14, 51, 44), "amber_dark")
    s.rect((9, 14, 51, 16), "amber_light")
    s.rect((9, 17, 11, 42), "amber")
    s.rect((12, 18, 50, 40), "ink")
    s.rect((14, 20, 48, 39), "shadow")
    s.rect((15, 21, 47, 22), "cyan_dark")
    s.numeral("67", 19, 23, "amber_dark", 2, 5)
    s.numeral("67", 18, 22, "amber_light", 2, 5)
    s.rect((17, 37, 22, 38), "cyan")
    s.rect((25, 37, 34, 38), "cyan_dark")
    s.rect((37, 37, 44, 38), "cyan")
    s.rect((12, 42, 36, 43), "amber")
    s.rect((39, 42, 42, 43), "lime_light")
    s.rect((45, 42, 48, 43), "cyan_light")
    for x, y in ((8, 16), (51, 16), (8, 41), (51, 41)):
        s.rect((x, y, x+1, y+1), "steel_light")
    s.line([(52, 22), (53, 22), (53, 35), (52, 35)], "amber")
    s.line([(51, 7), (54, 9)], "cyan")
    s.line([(54, 5), (58, 8)], "cyan_dark")
    return s.image


def nice_number():
    s = Sprite()
    # A pilot's knowing wink, seen through a substantial cyan glass visor.
    s.poly([(12, 51), (22, 45), (42, 45), (52, 51), (55, 58), (9, 58)], "ink")
    s.poly([(12, 52), (24, 47), (40, 47), (51, 52), (52, 56), (12, 56)], "steel")
    s.poly([(15, 51), (22, 49), (22, 55), (13, 55)], "steel_light")
    s.poly([(42, 49), (50, 53), (51, 56), (42, 55)], "steel_mid")
    s.rect((26, 50, 37, 56), "shadow")
    s.rect((28, 51, 35, 52), "amber")
    s.poly([(17, 13), (23, 8), (40, 8), (47, 14), (50, 26), (47, 43), (41, 49), (23, 49), (16, 43), (13, 26)], "ink")
    s.poly([(19, 14), (24, 10), (39, 10), (44, 14), (47, 25), (45, 41), (40, 46), (24, 46), (19, 41), (16, 25)], "steel_mid")
    s.poly([(19, 14), (24, 10), (37, 10), (38, 12), (25, 12), (21, 16), (19, 24), (17, 24)], "steel_light")
    s.poly([(40, 12), (44, 16), (47, 25), (44, 42), (39, 46), (35, 46), (39, 40), (42, 27)], "steel")
    s.rect((29, 10, 32, 17), "amber_light")
    s.rect((33, 10, 34, 17), "amber")
    s.poly([(18, 22), (23, 18), (42, 18), (46, 22), (45, 35), (39, 39), (24, 39), (18, 35)], "ink")
    s.poly([(20, 23), (24, 20), (41, 20), (44, 23), (43, 33), (39, 36), (25, 36), (20, 33)], "cyan_dark")
    s.poly([(21, 23), (25, 21), (40, 21), (42, 23), (42, 26), (21, 26)], "cyan")
    s.line([(23, 22), (26, 21), (32, 21)], "cyan_light")
    # One open pixel eye, one clearly sloped closed/winking eye.
    s.rect((25, 27, 27, 31), "white")
    s.rect((27, 27, 28, 29), "lime_light")
    s.line([(35, 29), (38, 27), (41, 29)], "lime_light", 2)
    s.line([(30, 33), (33, 34), (36, 33)], "cyan_light")
    s.rect((11, 25, 17, 36), "ink")
    s.rect((12, 26, 15, 34), "amber_dark")
    s.rect((12, 26, 14, 29), "amber_light")
    s.rect((47, 25, 52, 36), "ink")
    s.rect((48, 26, 50, 34), "steel_light")
    s.line([(17, 38), (22, 42), (27, 42)], "amber", 2)
    s.rect((26, 40, 30, 43), "ink")
    s.rect((27, 41, 29, 42), "amber_light")
    s.star(55, 17, "amber_light")
    s.rect((52, 56, 52, 56), "steel")
    return s.image


def result_found():
    s = Sprite()
    # Portable radar recovering a wandering signal outside its scan envelope.
    s.rect((15, 48, 48, 56), "ink")
    s.rect((17, 49, 46, 53), "steel")
    s.rect((18, 49, 29, 50), "steel_light")
    s.rect((34, 50, 36, 51), "lime_light")
    s.rect((39, 50, 43, 51), "amber")
    s.ellipse((8, 10, 53, 54), "ink")
    s.ellipse((10, 11, 51, 51), "steel_mid")
    s.ellipse((11, 12, 48, 48), "steel_light")
    s.ellipse((14, 15, 48, 49), "ink")
    s.ellipse((16, 17, 46, 47), "cyan_dark")
    s.ellipse((18, 19, 44, 45), "shadow")
    s.ellipse((21, 22, 41, 42), "cyan_dark")
    s.ellipse((22, 23, 40, 41), "shadow")
    s.ellipse((26, 27, 36, 37), "cyan_dark")
    s.ellipse((27, 28, 35, 36), "shadow")
    s.line([(31, 18), (31, 46)], "cyan_dark")
    s.line([(17, 32), (45, 32)], "cyan_dark")
    s.poly([(31, 32), (31, 18), (42, 23)], "cyan_dark")
    s.poly([(31, 32), (39, 22), (42, 24)], "cyan")
    s.line([(31, 32), (42, 23)], "cyan_light")
    s.rect((29, 30, 33, 34), "cyan")
    s.rect((30, 31, 32, 33), "cyan_light")
    s.rect((22, 36, 24, 38), "lime")
    s.rect((36, 27, 38, 29), "lime_dark")
    # A broken outbound cable and the recovered beacon are recognisable apart.
    s.line([(46, 22), (50, 18), (53, 18)], "amber_dark", 2)
    s.line([(50, 21), (53, 21)], "amber")
    s.poly([(54, 12), (59, 17), (54, 22), (49, 17)], "ink")
    s.poly([(54, 13), (58, 17), (54, 21), (50, 17)], "amber")
    s.poly([(54, 14), (56, 17), (54, 18), (52, 17)], "amber_light")
    s.line([(49, 10), (47, 8)], "amber_light")
    s.line([(59, 11), (61, 9)], "amber_light")
    s.rect((55, 6, 56, 8), "amber_light")
    s.rect((10, 29, 12, 31), "cyan_light")
    s.rect((31, 12, 33, 14), "cyan_light")
    return s.image


def peer_review():
    s = Sprite()
    # Reviewed dossier held in a control clamp, with a physical approval seal.
    s.poly([(14, 7), (38, 7), (49, 18), (49, 53), (14, 53)], "ink")
    s.poly([(16, 9), (37, 9), (47, 19), (47, 50), (16, 50)], "steel_light")
    s.poly([(17, 10), (36, 10), (45, 19), (45, 48), (17, 48)], "paper")
    s.rect((17, 10, 19, 48), "white")
    s.poly([(37, 10), (37, 19), (45, 19)], "steel_mid")
    s.poly([(38, 11), (38, 17), (44, 17)], "steel_light")
    s.rect((22, 15, 32, 17), "steel")
    s.rect((22, 21, 38, 22), "steel_mid")
    s.rect((22, 25, 34, 26), "steel_mid")
    s.rect((22, 29, 37, 30), "steel_mid")
    s.rect((22, 34, 28, 35), "steel_mid")
    s.rect((22, 39, 30, 40), "steel_mid")
    s.rect((10, 20, 16, 45), "ink")
    s.rect((11, 22, 14, 42), "steel")
    s.rect((11, 22, 12, 37), "steel_light")
    s.rect((12, 25, 17, 28), "cyan")
    s.rect((12, 36, 17, 39), "cyan_dark")
    s.rect((18, 50, 42, 54), "ink")
    s.rect((20, 50, 40, 52), "steel")
    # The inspection lens overlaps the paper; the stamp is not floating decoration.
    s.line([(45, 42), (53, 52)], "ink", 7)
    s.line([(46, 43), (53, 52)], "amber_dark", 4)
    s.line([(48, 45), (53, 51)], "amber", 2)
    s.ellipse((29, 27, 52, 49), "ink")
    s.ellipse((31, 29, 50, 47), "amber")
    s.ellipse((32, 30, 48, 45), "amber_light")
    s.ellipse((34, 32, 48, 45), "lime_dark")
    s.ellipse((35, 33, 46, 43), "lime")
    s.line([(37, 37), (40, 40), (45, 35)], "shadow", 2)
    s.line([(36, 36), (39, 39), (44, 34)], "white", 2)
    s.rect((35, 32, 38, 32), "lime_light")
    s.rect((6, 13, 8, 15), "cyan")
    s.line([(53, 25), (55, 23), (57, 23)], "cyan_light")
    return s.image


def bracket_architect():
    s = Sprite()
    # Nested physical bracket portals recede into a tiny contained world.
    s.poly([(8, 52), (31, 60), (57, 50), (34, 43)], "ink")
    s.poly([(10, 52), (31, 57), (54, 49), (33, 46)], "steel")
    s.line([(11, 52), (31, 57), (53, 50)], "steel_light")
    # Each bracket has a light front face, a darker inward wall, and a shadow foot.
    s.poly([(6, 14), (21, 9), (21, 15), (13, 18), (13, 43), (21, 46), (21, 52), (6, 47)], "ink")
    s.poly([(8, 15), (20, 11), (20, 14), (11, 17), (11, 44), (20, 47), (20, 50), (8, 46)], "cyan")
    s.line([(8, 15), (20, 11)], "cyan_light", 2)
    s.rect((8, 17, 9, 44), "cyan_light")
    s.poly([(11, 18), (14, 19), (14, 42), (11, 44)], "cyan_dark")
    s.poly([(44, 9), (58, 14), (58, 47), (44, 52), (44, 46), (51, 43), (51, 18), (44, 15)], "ink")
    s.poly([(45, 11), (56, 15), (56, 46), (45, 50), (45, 47), (53, 44), (53, 17), (45, 14)], "cyan_dark")
    s.line([(45, 11), (56, 15), (56, 44)], "cyan")
    s.line([(45, 11), (54, 14)], "cyan_light")
    s.poly([(18, 22), (27, 18), (27, 24), (24, 25), (24, 37), (27, 39), (27, 45), (18, 40)], "ink")
    s.poly([(20, 23), (26, 20), (26, 23), (22, 25), (22, 38), (26, 40), (26, 43), (20, 39)], "lime")
    s.line([(20, 23), (26, 20)], "lime_light", 2)
    s.rect((20, 24, 21, 38), "lime_light")
    s.poly([(38, 18), (47, 22), (47, 40), (38, 45), (38, 39), (41, 37), (41, 25), (38, 24)], "ink")
    s.poly([(39, 20), (45, 23), (45, 39), (39, 43), (39, 40), (43, 38), (43, 25), (39, 23)], "lime_dark")
    s.line([(39, 20), (45, 23), (45, 38)], "lime")
    s.poly([(28, 29), (33, 26), (38, 29), (38, 36), (33, 40), (28, 36)], "ink")
    s.poly([(29, 29), (33, 27), (37, 29), (33, 32)], "amber_light")
    s.poly([(29, 30), (33, 33), (33, 38), (29, 35)], "amber")
    s.poly([(34, 33), (37, 30), (37, 35), (34, 38)], "amber_dark")
    s.star(31, 8, "amber_light")
    s.rect((31, 46, 33, 48), "amber")
    return s.image


def scientific_method():
    s = Sprite()
    # A laboratory microscope, articulated arm and illuminated specimen slide.
    s.poly([(12, 51), (46, 51), (53, 57), (53, 59), (8, 59), (8, 56)], "ink")
    s.poly([(13, 53), (45, 53), (50, 57), (10, 57)], "steel_mid")
    s.line([(14, 53), (44, 53), (47, 55)], "steel_light")
    s.rect((16, 57, 48, 58), "steel")
    s.rect((20, 54, 26, 55), "amber_light")
    s.rect((39, 46, 46, 53), "ink")
    s.rect((40, 47, 44, 52), "steel")
    s.poly([(39, 15), (46, 18), (51, 26), (51, 42), (45, 49), (38, 49), (36, 43), (43, 41), (45, 36), (45, 29), (40, 23), (36, 22)], "ink")
    s.poly([(40, 18), (44, 20), (48, 27), (48, 40), (44, 46), (39, 46), (38, 44), (44, 42), (46, 37), (46, 28), (41, 22), (38, 21)], "cyan_dark")
    s.line([(40, 18), (44, 20), (48, 27), (48, 37)], "cyan", 2)
    s.line([(41, 18), (45, 21)], "cyan_light")
    # Angled optical tube with three-toned walls and a chunky eyepiece.
    s.poly([(18, 9), (25, 5), (42, 24), (33, 32)], "ink")
    s.poly([(20, 10), (25, 7), (39, 24), (33, 29)], "steel_mid")
    s.poly([(20, 10), (23, 9), (36, 25), (33, 27)], "steel_light")
    s.poly([(25, 9), (38, 24), (36, 26), (23, 10)], "steel")
    s.poly([(15, 9), (24, 3), (28, 7), (19, 14)], "ink")
    s.poly([(17, 9), (24, 5), (25, 7), (19, 12)], "cyan")
    s.line([(17, 9), (24, 5)], "cyan_light")
    s.poly([(32, 28), (36, 25), (39, 28), (35, 33)], "ink")
    s.line([(33, 29), (36, 27), (37, 28), (34, 31)], "amber")
    s.rect((13, 38, 41, 42), "ink")
    s.rect((14, 38, 40, 39), "steel_light")
    s.rect((15, 40, 40, 41), "steel")
    s.rect((22, 36, 33, 37), "cyan_light")
    s.rect((24, 35, 31, 35), "cyan")
    s.rect((26, 34, 29, 35), "lime_light")
    s.ellipse((41, 29, 52, 40), "ink")
    s.ellipse((43, 31, 50, 38), "amber_dark")
    s.ellipse((43, 31, 48, 36), "amber")
    s.rect((45, 32, 47, 34), "amber_light")
    # Small amber reagent bottle balances the microscope's right-hand arm.
    s.rect((7, 42, 17, 54), "ink")
    s.rect((10, 38, 14, 43), "ink")
    s.rect((10, 39, 14, 40), "amber_light")
    s.rect((9, 44, 15, 52), "amber_dark")
    s.rect((9, 46, 15, 51), "amber")
    s.rect((10, 44, 11, 49), "amber_light")
    s.rect((52, 14, 54, 16), "lime")
    s.star(10, 24, "cyan_light")
    return s.image


def touch_grass():
    s = Sprite()
    # A living island, not a lawn glyph: soil strata, varied blades and field beacon.
    s.poly([(7, 43), (12, 39), (50, 39), (57, 44), (54, 51), (44, 57), (19, 57), (9, 51)], "ink")
    s.poly([(10, 44), (17, 41), (48, 41), (54, 44), (51, 49), (43, 54), (21, 54), (12, 49)], "amber_dark")
    s.poly([(12, 45), (24, 46), (26, 54), (21, 54), (13, 49)], "steel")
    s.poly([(39, 46), (51, 45), (48, 50), (42, 54), (36, 54)], "shadow")
    s.rect((17, 48, 19, 49), "amber")
    s.rect((28, 50, 31, 51), "amber")
    s.rect((39, 49, 41, 50), "amber")
    s.rect((23, 54, 25, 56), "amber_dark")
    s.rect((35, 54, 36, 57), "steel")
    s.poly([(8, 40), (14, 36), (23, 34), (42, 34), (51, 38), (57, 42), (49, 46), (21, 47), (9, 44)], "lime_dark")
    s.poly([(11, 39), (24, 35), (42, 36), (53, 41), (47, 43), (20, 44), (11, 42)], "lime")
    # Draw individual authored blades, with dark roots and lit tips.
    blades = [
        (12, 41, 8, 30), (16, 40, 14, 25), (20, 40, 25, 29),
        (23, 39, 20, 22), (27, 41, 29, 29), (32, 41, 30, 24),
        (37, 41, 40, 29), (44, 42, 46, 25), (49, 43, 54, 31),
        (53, 41, 58, 35), (16, 43, 18, 33), (41, 44, 38, 34),
    ]
    for index, (x, root, tip_x, tip_y) in enumerate(blades):
        s.poly([(x-2, root), (tip_x, tip_y), (x+1, root-4), (x+2, root)], "lime_dark")
        s.poly([(x-1, root-1), (tip_x, tip_y), (x+1, root-3)], "lime" if index % 3 == 0 else "lime_light")
    # Explicit cutaways preserve the original blade silhouette on Pillow 10+.
    for x, y in ((47, 25), (41, 29), (55, 31), (19, 33)):
        s.rect((x, y, x, y), "#00000000")
    for x, y in ((25, 40), (14, 42)):
        s.rect((x, y, x, y), "lime_dark")
    s.line([(12, 44), (22, 46), (36, 45)], "lime_dark")
    s.rect((18, 43, 20, 44), "lime_light")
    s.rect((46, 42, 48, 43), "lime_light")
    # Solar-powered expedition beacon gives this organic icon its sci-fi identity.
    s.rect((33, 22, 38, 42), "ink")
    s.rect((34, 23, 35, 40), "steel_light")
    s.rect((36, 23, 37, 40), "steel")
    s.poly([(30, 17), (34, 12), (39, 12), (43, 17), (43, 24), (30, 24)], "ink")
    s.poly([(32, 17), (35, 14), (38, 14), (41, 17), (41, 22), (32, 22)], "amber")
    s.rect((33, 17, 36, 21), "amber_light")
    s.rect((38, 17, 40, 21), "amber_dark")
    s.rect((29, 23, 44, 25), "ink")
    s.rect((31, 23, 42, 24), "steel_light")
    s.rect((35, 8, 37, 12), "steel")
    s.rect((36, 8, 36, 10), "cyan_light")
    s.line([(26, 14), (23, 11)], "amber_light")
    s.line([(47, 14), (50, 11)], "amber_light")
    s.rect((36, 3, 37, 5), "amber_light")
    # One tiny field flower and a drifting firefly avoid a mechanical-only set.
    s.rect((8, 23, 8, 27), "lime_dark")
    s.star(8, 22, "amber_light", 1)
    s.rect((55, 23, 56, 24), "cyan_light")
    return s.image


def second_wind():
    s = Sprite()
    # New life pushes through a ruptured power cell, not another grass island.
    s.poly([(12, 36), (23, 31), (48, 34), (53, 40), (49, 54), (39, 59), (17, 55), (10, 47)], "ink")
    s.poly([(13, 38), (24, 34), (46, 36), (50, 41), (47, 52), (38, 56), (18, 53), (13, 47)], "amber_dark")
    s.poly([(13, 38), (24, 34), (46, 36), (39, 42), (18, 42)], "amber")
    s.line([(14, 38), (24, 35), (36, 36)], "amber_light", 2)
    s.poly([(18, 43), (37, 45), (37, 55), (19, 52)], "steel")
    s.line([(20, 44), (20, 50), (33, 53)], "steel_light")
    s.poly([(40, 43), (48, 40), (46, 51), (40, 54)], "shadow")
    s.rect((24, 46, 31, 48), "cyan")
    s.rect((26, 44, 28, 51), "cyan_light")
    s.line([(38, 37), (34, 41), (38, 45), (35, 51), (38, 56)], "ink", 2)
    s.poly([(22, 35), (27, 31), (33, 32), (37, 37), (33, 40), (26, 39)], "ink")
    s.line([(29, 37), (29, 26), (33, 17)], "lime_dark", 4)
    s.line([(29, 36), (29, 27), (33, 18)], "lime_light", 2)
    s.poly([(30, 27), (22, 28), (16, 24), (13, 17), (21, 17), (28, 21)], "ink")
    s.poly([(29, 25), (22, 26), (18, 23), (15, 19), (22, 19), (27, 22)], "lime")
    s.line([(17, 19), (23, 22), (29, 26)], "lime_light")
    s.poly([(31, 20), (31, 12), (37, 7), (46, 6), (43, 15), (38, 20)], "ink")
    s.poly([(33, 18), (33, 13), (38, 9), (43, 8), (41, 14), (37, 18)], "lime")
    s.line([(34, 17), (39, 12), (42, 9)], "lime_light", 2)
    s.line([(8, 30), (5, 28)], "amber")
    s.line([(49, 29), (55, 26)], "cyan")
    s.star(51, 13, "cyan_light", 1)
    s.rect((8, 51, 10, 52), "steel_mid")
    return s.image


def alternate_routes():
    s = Sprite()
    # Three visibly separate circuit roads meet at one destination beacon.
    s.poly([(6, 49), (12, 42), (22, 47), (28, 37), (28, 28), (21, 22),
            (9, 24), (7, 18), (22, 15), (32, 22), (41, 15), (54, 17),
            (53, 23), (42, 22), (36, 28), (36, 37), (46, 45), (56, 42),
            (59, 48), (45, 53), (32, 42), (24, 54), (13, 49), (9, 54)], "ink")
    s.line([(9, 49), (13, 45), (23, 50), (32, 38), (32, 24)], "steel_mid", 4)
    s.line([(10, 20), (22, 18), (32, 26)], "cyan", 4)
    s.line([(55, 20), (42, 18), (32, 26)], "amber", 4)
    s.line([(55, 46), (46, 49), (32, 38)], "lime", 4)
    s.line([(10, 19), (21, 17), (30, 24)], "cyan_light")
    s.line([(43, 17), (54, 19)], "amber_light")
    s.line([(35, 39), (46, 48), (54, 45)], "lime_light")
    s.rect((31, 28, 32, 30), "white")
    s.rect((31, 34, 32, 36), "white")
    s.rect((16, 45, 18, 46), "paper")
    s.rect((23, 47, 24, 48), "paper")
    s.rect((30, 7, 35, 23), "ink")
    s.rect((31, 8, 33, 22), "steel_light")
    s.rect((34, 9, 34, 22), "steel")
    s.poly([(27, 5), (37, 5), (41, 10), (37, 15), (27, 15), (23, 10)], "ink")
    s.poly([(28, 7), (36, 7), (38, 10), (36, 13), (28, 13), (26, 10)], "lime")
    s.rect((29, 8, 35, 10), "lime_light")
    s.rect((4, 17, 7, 22), "steel")
    s.rect((56, 17, 59, 22), "steel")
    s.star(48, 7, "cyan_light", 1)
    return s.image


def quiet_after_storm():
    s = Sprite()
    # A rain-filled impact crater: a calm zero-shaped pool below a broken storm.
    s.poly([(9, 12), (13, 8), (21, 8), (25, 12), (33, 12), (37, 16),
            (36, 20), (8, 20), (5, 17)], "ink")
    s.poly([(10, 13), (14, 10), (20, 10), (24, 14), (32, 14), (35, 17),
            (34, 18), (8, 18), (8, 15)], "steel")
    s.line([(11, 12), (15, 10), (19, 10)], "steel_light")
    s.poly([(25, 19), (20, 27), (25, 27), (22, 33), (31, 24), (26, 24), (30, 19)], "amber_dark")
    s.line([(11, 23), (9, 27)], "cyan_dark")
    s.line([(35, 23), (33, 26)], "cyan_dark")
    s.poly([(5, 44), (12, 36), (22, 33), (27, 36), (36, 33), (48, 36),
            (59, 45), (55, 53), (43, 58), (19, 58), (8, 52)], "ink")
    s.poly([(8, 44), (14, 38), (23, 35), (28, 38), (36, 36), (46, 38),
            (56, 45), (52, 51), (42, 55), (20, 55), (11, 50)], "steel")
    s.poly([(9, 43), (15, 38), (23, 36), (27, 40), (21, 42), (15, 46)], "steel_light")
    s.poly([(39, 38), (46, 39), (53, 45), (48, 47), (39, 42)], "steel_mid")
    s.ellipse((13, 39, 51, 53), "shadow")
    s.ellipse((16, 41, 48, 51), "cyan_dark")
    s.ellipse((19, 42, 45, 49), "cyan")
    s.ellipse((25, 44, 39, 47), "shadow")
    s.line([(20, 43), (26, 42), (36, 42)], "cyan_light")
    s.line([(38, 49), (44, 47)], "cyan_light")
    s.rect((16, 52, 20, 53), "steel_mid")
    s.rect((44, 53, 47, 54), "steel_light")
    s.star(48, 16, "paper", 1)
    s.rect((53, 22, 54, 23), "cyan")
    return s.image


def paper_tiger():
    s = Sprite()
    # An actual folded-paper tiger: sharp ears, folded muzzle, stripes and tail.
    s.line([(45, 37), (54, 34), (57, 25), (54, 20)], "ink", 5)
    s.line([(46, 37), (53, 34), (55, 25), (53, 21)], "amber", 3)
    s.rect((52, 20, 55, 23), "amber_light")
    s.poly([(18, 28), (38, 24), (48, 34), (44, 46), (38, 50),
            (36, 56), (31, 56), (31, 46), (23, 45), (19, 54), (13, 54), (15, 38)], "ink")
    s.poly([(19, 30), (37, 27), (45, 35), (42, 44), (36, 47),
            (35, 54), (33, 54), (33, 43), (22, 42), (18, 52), (16, 52), (18, 38)], "amber")
    s.poly([(20, 31), (35, 29), (29, 39), (19, 39)], "amber_light")
    s.poly([(29, 39), (37, 28), (44, 35), (39, 43)], "amber_dark")
    s.poly([(24, 30), (27, 29), (24, 37), (21, 38)], "ink")
    s.poly([(34, 29), (37, 29), (33, 36), (30, 38)], "ink")
    s.poly([(41, 35), (44, 36), (40, 41), (38, 41)], "ink")
    s.poly([(10, 12), (19, 17), (25, 13), (29, 7), (32, 23), (27, 33),
            (17, 35), (7, 26), (8, 19)], "ink")
    s.poly([(12, 15), (19, 20), (25, 16), (28, 12), (29, 23),
            (25, 30), (18, 32), (10, 25), (10, 20)], "amber_light")
    s.poly([(19, 20), (28, 16), (29, 23), (25, 30), (19, 27)], "amber")
    s.poly([(10, 21), (16, 23), (14, 25), (10, 24)], "ink")
    s.poly([(24, 20), (28, 18), (27, 22), (24, 23)], "ink")
    s.rect((15, 24, 17, 25), "shadow")
    s.rect((23, 23, 25, 24), "shadow")
    s.poly([(15, 27), (19, 25), (23, 27), (20, 31), (17, 30)], "paper")
    s.rect((18, 27, 20, 28), "ink")
    s.line([(19, 29), (19, 31)], "amber_dark")
    s.line([(20, 34), (24, 40), (22, 43)], "white")
    s.rect((12, 56, 19, 57), "shadow")
    s.rect((31, 58, 38, 59), "shadow")
    s.star(45, 12, "cyan_light", 1)
    return s.image


def gaining_altitude():
    s = Sprite()
    # A little expedition rocket leaves five ever-higher launch terraces.
    s.poly([(5, 51), (15, 51), (15, 43), (25, 43), (25, 35), (35, 35),
            (35, 27), (45, 27), (45, 19), (56, 19), (56, 58), (5, 58)], "ink")
    for index in range(5):
        x, y = 7 + index*10, 52 - index*8
        s.rect((x, y, x+7, 56), "steel")
        s.rect((x, y, x+7, y+1), "steel_light")
        s.rect((x+1, y+3, x+3, y+4), "cyan_dark")
    s.poly([(31, 26), (35, 17), (39, 13), (43, 20), (44, 28),
            (41, 36), (35, 35)], "ink")
    s.poly([(33, 26), (36, 18), (39, 16), (41, 21), (42, 28),
            (39, 33), (36, 32)], "paper")
    s.poly([(39, 16), (41, 21), (42, 28), (39, 33), (38, 27)], "steel_mid")
    s.poly([(33, 25), (28, 32), (27, 38), (35, 33)], "ink")
    s.poly([(33, 28), (30, 33), (30, 35), (35, 31)], "cyan")
    s.poly([(42, 26), (47, 33), (46, 39), (40, 33)], "ink")
    s.poly([(42, 29), (45, 34), (45, 36), (41, 32)], "cyan_dark")
    s.ellipse((35, 23, 40, 28), "ink")
    s.rect((36, 24, 38, 26), "cyan_light")
    s.poly([(35, 35), (40, 36), (37, 43), (32, 48), (33, 40)], "amber")
    s.poly([(36, 35), (38, 36), (35, 43)], "amber_light")
    s.line([(26, 44), (22, 48)], "amber_dark", 2)
    s.line([(18, 34), (25, 27), (28, 19)], "cyan_dark")
    s.star(17, 18, "lime_light", 1)
    s.star(51, 6, "amber_light", 2)
    return s.image


def mirror_room():
    s = Sprite()
    # A hinged standing mirror repeats the same orb on opposing silver planes.
    s.poly([(5, 13), (29, 5), (32, 9), (58, 16), (58, 50),
            (34, 58), (30, 55), (5, 48)], "ink")
    s.poly([(7, 15), (28, 8), (29, 52), (7, 46)], "amber")
    s.poly([(10, 17), (26, 12), (27, 47), (10, 43)], "shadow")
    s.poly([(11, 18), (24, 14), (25, 44), (11, 41)], "steel")
    s.poly([(34, 12), (55, 18), (55, 48), (34, 55)], "amber_dark")
    s.poly([(37, 17), (52, 21), (52, 45), (37, 50)], "shadow")
    s.poly([(38, 18), (50, 22), (50, 43), (38, 47)], "cyan_dark")
    s.line([(8, 15), (27, 9)], "amber_light")
    s.line([(35, 13), (54, 18)], "amber_light")
    s.rect((30, 10, 32, 54), "steel")
    s.rect((30, 12, 30, 51), "steel_light")
    s.rect((31, 17, 33, 20), "amber")
    s.rect((31, 43, 33, 46), "amber")
    s.ellipse((13, 25, 24, 36), "ink")
    s.ellipse((14, 26, 23, 34), "lime")
    s.rect((15, 27, 18, 29), "lime_light")
    s.ellipse((39, 26, 50, 37), "ink")
    s.ellipse((40, 27, 49, 35), "lime_dark")
    s.rect((46, 28, 48, 30), "lime")
    s.rect((14, 20, 20, 21), "paper")
    s.rect((17, 18, 18, 23), "paper")
    s.rect((41, 22, 47, 23), "cyan_light")
    s.line([(12, 37), (21, 18)], "steel_light")
    s.line([(41, 44), (49, 26)], "cyan")
    s.poly([(9, 48), (15, 50), (13, 55), (8, 55)], "ink")
    s.poly([(49, 51), (54, 49), (57, 54), (52, 56)], "ink")
    s.star(57, 7, "paper", 1)
    return s.image


def trouble_collector():
    s = Sprite()
    # Three different error specimens, individually bottled and labelled.
    s.poly([(5, 44), (56, 44), (59, 51), (56, 58), (7, 58), (4, 51)], "ink")
    s.rect((7, 46, 55, 54), "steel")
    s.rect((8, 46, 54, 47), "steel_light")
    s.rect((8, 55, 54, 56), "shadow")
    for x, tone, light in ((8, "amber", "amber_light"),
                           (25, "cyan", "cyan_light"),
                           (42, "lime", "lime_light")):
        s.rect((x, 17, x+13, 48), "ink")
        s.rect((x+2, 19, x+11, 45), "steel")
        s.rect((x+3, 20, x+10, 43), "shadow")
        s.rect((x+3, 34, x+10, 43), tone)
        s.rect((x+3, 34, x+10, 35), light)
        s.rect((x+3, 20, x+3, 31), "steel_light")
        s.rect((x+1, 12, x+12, 18), "ink")
        s.rect((x+2, 13, x+11, 15), "steel_light")
        s.rect((x+2, 16, x+11, 17), "steel_mid")
        s.rect((x+4, 49, x+9, 51), tone)
    # Zig-zag syntax shard, impossible division ring, spiky domain organism.
    s.line([(14, 23), (11, 27), (16, 27), (13, 31)], "amber_light", 2)
    s.ellipse((29, 23, 35, 29), "cyan")
    s.ellipse((31, 25, 33, 27), "shadow")
    s.line([(28, 30), (36, 22)], "cyan_light")
    s.rect((47, 24, 50, 29), "lime")
    s.rect((45, 26, 52, 27), "lime_light")
    s.rect((48, 22, 49, 31), "lime_light")
    s.rect((48, 26, 49, 27), "ink")
    s.line([(5, 8), (9, 6)], "amber_dark")
    s.star(55, 6, "cyan_light", 1)
    s.rect((35, 7, 36, 8), "lime")
    return s.image


def unscathed():
    s = Sprite()
    # A pristine shield; deflected chips remain outside its clean front face.
    s.poly([(12, 13), (31, 5), (51, 13), (49, 37), (42, 48),
            (31, 58), (20, 49), (13, 38)], "ink")
    s.poly([(14, 15), (31, 8), (48, 15), (46, 36), (40, 46),
            (31, 54), (22, 46), (16, 36)], "steel_light")
    s.poly([(18, 18), (31, 12), (44, 18), (42, 35), (37, 43),
            (31, 49), (25, 43), (20, 35)], "cyan_dark")
    s.poly([(19, 19), (30, 14), (30, 45), (25, 40), (22, 34)], "cyan")
    s.line([(16, 16), (31, 10), (46, 16)], "white")
    s.line([(16, 19), (18, 35), (24, 44)], "paper")
    s.poly([(25, 25), (31, 21), (37, 25), (35, 35), (31, 40), (27, 35)], "ink")
    s.poly([(27, 26), (31, 23), (35, 26), (33, 34), (31, 37), (29, 34)], "lime")
    s.line([(28, 27), (31, 25), (33, 27)], "lime_light", 2)
    s.star(40, 17, "white", 2)
    s.poly([(4, 25), (8, 23), (10, 28), (7, 30)], "amber")
    s.line([(3, 19), (7, 20)], "amber_dark", 2)
    s.poly([(53, 35), (58, 32), (60, 36), (56, 40)], "amber_dark")
    s.line([(53, 43), (57, 46)], "amber")
    s.rect((8, 45, 10, 47), "steel_mid")
    s.rect((47, 53, 49, 54), "cyan")
    return s.image


def grand_scale():
    s = Sprite()
    # A tilted spiral galaxy, with hand-placed dust clusters and orbital depth.
    s.line([(7, 40), (9, 29), (18, 20), (31, 16), (44, 18), (52, 24),
            (54, 32), (48, 40), (39, 46), (25, 49), (14, 45)], "ink", 6)
    s.line([(8, 39), (10, 30), (19, 22), (31, 18), (43, 20), (50, 25),
            (51, 32), (47, 38), (38, 44), (25, 47), (15, 43)], "cyan_dark", 4)
    s.line([(10, 35), (18, 25), (31, 21), (42, 23), (46, 28),
            (43, 35), (35, 40), (25, 41), (20, 37), (23, 31),
            (32, 27), (38, 28), (38, 32), (32, 35)], "cyan", 3)
    s.line([(12, 32), (20, 25), (31, 22), (39, 23)], "cyan_light")
    s.line([(26, 40), (35, 38), (42, 33)], "paper")
    s.line([(14, 47), (22, 51), (38, 49), (49, 43), (58, 32)], "steel", 2)
    s.line([(9, 26), (17, 18), (32, 13), (47, 16)], "steel_mid")
    s.ellipse((26, 26, 39, 38), "amber_dark")
    s.ellipse((27, 26, 37, 35), "amber")
    s.ellipse((29, 27, 35, 32), "amber_light")
    s.rect((30, 28, 33, 30), "white")
    for x, y, color in ((16, 29, "white"), (24, 23, "lime_light"),
                        (44, 26, "cyan_light"), (46, 37, "amber_light"),
                        (34, 43, "cyan_light"), (19, 43, "lime"),
                        (52, 29, "paper"), (40, 19, "amber")):
        s.rect((x, y, x+1, y+1), color)
    s.star(9, 12, "cyan_light", 2)
    s.star(53, 9, "amber_light", 1)
    s.star(55, 53, "lime_light", 2)
    s.rect((5, 51, 6, 52), "paper")
    return s.image


def last_pixel():
    s = Sprite()
    # Precision caliper jaws surround one actual luminous pixel, not a big gem.
    s.rect((7, 9, 54, 18), "ink")
    s.rect((9, 11, 52, 16), "steel_mid")
    s.rect((10, 11, 51, 12), "steel_light")
    for x in range(13, 51, 6):
        s.rect((x, 13, x, 15), "paper")
    s.poly([(9, 16), (17, 16), (17, 30), (27, 30), (27, 35),
            (12, 35), (9, 32)], "ink")
    s.poly([(11, 17), (15, 17), (15, 32), (25, 32), (25, 33),
            (13, 33), (11, 31)], "steel_light")
    s.poly([(39, 16), (49, 16), (49, 33), (46, 36), (36, 36), (36, 31),
            (41, 31), (41, 22), (39, 22)], "ink")
    s.poly([(43, 18), (47, 18), (47, 32), (45, 34), (38, 34),
            (38, 33), (43, 33)], "cyan")
    s.rect((38, 8, 48, 21), "ink")
    s.rect((40, 10, 46, 19), "steel")
    s.rect((40, 10, 41, 17), "paper")
    s.rect((43, 12, 45, 15), "amber")
    # Alignment ticks end two pixels clear of the one-pixel specimen.
    s.line([(31, 23), (31, 29)], "cyan_dark")
    s.line([(31, 35), (31, 42)], "cyan_dark")
    s.line([(21, 32), (28, 32)], "cyan_dark")
    s.line([(34, 32), (41, 32)], "cyan_dark")
    s.rect((31, 32, 31, 32), "lime_light")
    s.line([(29, 40), (24, 48), (13, 48)], "steel_mid")
    s.rect((10, 45, 17, 54), "ink")
    s.rect((12, 47, 15, 52), "cyan")
    s.rect((12, 47, 13, 49), "cyan_light")
    s.rect((38, 48, 51, 55), "ink")
    s.rect((40, 50, 49, 53), "steel")
    s.rect((43, 51, 47, 51), "lime")
    s.star(55, 39, "paper", 1)
    return s.image


def parallel_worlds():
    s = Sprite()
    # Two independent oval gates share a travelling moon and an amber conduit.
    s.line([(15, 50), (23, 57), (39, 57), (49, 51)], "ink", 4)
    s.line([(16, 50), (24, 55), (39, 55), (48, 51)], "amber", 2)
    for x, y, face, bright in ((6, 10, "cyan", "cyan_light"),
                                (36, 15, "lime", "lime_light")):
        s.ellipse((x, y, x+21, y+42), "ink")
        s.ellipse((x+2, y+2, x+19, y+39), face)
        s.ellipse((x+4, y+4, x+17, y+36), "shadow")
        s.ellipse((x+6, y+6, x+16, y+34), "cyan_dark")
        s.ellipse((x+7, y+7, x+15, y+33), "shadow")
        s.line([(x+5, y+8), (x+3, y+16), (x+3, y+25)], bright)
        s.rect((x+8, y+3, x+12, y+4), bright)
        s.rect((x+14, y+32, x+16, y+34), bright)
        s.rect((x+5, y+39, x+16, y+43), "ink")
        s.rect((x+6, y+39, x+15, y+40), "steel_light")
    s.ellipse((12, 25, 22, 35), "ink")
    s.ellipse((13, 26, 21, 33), "paper")
    s.rect((14, 27, 17, 28), "white")
    s.rect((18, 30, 20, 32), "steel_mid")
    s.ellipse((41, 30, 51, 40), "ink")
    s.ellipse((42, 31, 50, 38), "paper")
    s.rect((43, 32, 46, 33), "white")
    s.rect((47, 35, 49, 37), "steel_mid")
    s.line([(24, 28), (28, 28), (28, 33), (33, 33)], "amber")
    s.rect((30, 31, 31, 35), "amber_light")
    s.star(32, 6, "amber_light", 1)
    s.rect((3, 55, 4, 56), "cyan")
    return s.image


def time_loop():
    s = Sprite()
    # Sand runs through a physical hourglass encircled by a returning arrow.
    s.line([(14, 48), (7, 40), (6, 28), (11, 17), (18, 12)], "ink", 5)
    s.line([(14, 48), (9, 40), (8, 29), (12, 19), (18, 14)], "cyan", 2)
    s.line([(47, 15), (55, 24), (56, 36), (52, 47), (44, 53)], "ink", 5)
    s.line([(47, 15), (53, 24), (54, 36), (50, 46), (44, 51)], "cyan", 2)
    s.poly([(14, 10), (23, 11), (19, 20)], "ink")
    s.poly([(16, 12), (21, 12), (19, 17)], "cyan_light")
    s.poly([(48, 46), (48, 56), (38, 54)], "ink")
    s.poly([(46, 49), (46, 54), (41, 53)], "cyan_light")
    s.rect((18, 6, 45, 12), "ink")
    s.rect((20, 8, 43, 10), "amber")
    s.rect((20, 8, 40, 8), "amber_light")
    s.rect((18, 51, 45, 57), "ink")
    s.rect((20, 53, 43, 55), "amber")
    s.rect((20, 53, 40, 53), "amber_light")
    s.poly([(21, 12), (42, 12), (41, 21), (34, 30), (34, 33),
            (41, 42), (42, 51), (21, 51), (22, 42), (29, 33), (29, 30), (22, 21)], "ink")
    s.poly([(23, 13), (40, 13), (39, 21), (32, 29), (32, 34),
            (39, 43), (40, 50), (23, 50), (24, 43), (31, 34), (31, 29), (24, 21)], "steel")
    s.poly([(25, 15), (38, 15), (37, 21), (32, 27), (30, 27), (26, 21)], "cyan_dark")
    s.poly([(26, 19), (37, 19), (34, 23), (31, 26), (28, 23)], "amber")
    s.rect((27, 19, 36, 20), "amber_light")
    s.rect((31, 30, 31, 35), "amber_light")
    s.rect((31, 38, 31, 39), "amber")
    s.poly([(25, 48), (31, 41), (38, 48)], "amber")
    s.line([(26, 47), (31, 42), (36, 47)], "amber_light")
    s.line([(24, 15), (25, 21), (29, 26)], "cyan_light")
    s.line([(24, 46), (26, 41), (29, 37)], "cyan")
    return s.image


def fourth_wall():
    s = Sprite()
    # A console window becomes a literal open window with a waving pixel hand.
    s.rect((7, 9, 54, 49), "ink")
    s.rect((9, 11, 52, 47), "steel_mid")
    s.rect((9, 11, 52, 17), "steel_light")
    s.rect((12, 13, 14, 15), "amber")
    s.rect((17, 13, 19, 15), "lime")
    s.rect((22, 13, 24, 15), "cyan")
    s.rect((11, 19, 50, 44), "shadow")
    s.rect((13, 21, 48, 42), "cyan_dark")
    s.rect((15, 23, 46, 40), "shadow")
    s.line([(16, 25), (23, 25), (26, 28), (23, 31), (16, 31)], "cyan_light")
    s.rect((17, 35, 26, 36), "cyan")
    # Hinged window pane projects out beyond the interface.
    s.poly([(47, 20), (58, 25), (58, 51), (47, 44)], "ink")
    s.poly([(49, 23), (56, 26), (56, 47), (49, 43)], "steel_light")
    s.poly([(50, 26), (54, 28), (54, 43), (50, 41)], "cyan_dark")
    s.rect((49, 34, 51, 36), "amber_light")
    s.poly([(27, 54), (27, 39), (23, 34), (23, 28), (26, 27), (30, 32),
            (30, 20), (34, 20), (34, 29), (35, 16), (39, 16), (39, 29),
            (41, 21), (45, 22), (43, 35), (43, 44), (39, 49), (39, 55)], "ink")
    s.poly([(29, 51), (29, 38), (25, 33), (25, 30), (27, 31), (32, 36),
            (32, 22), (32, 31), (36, 32), (37, 18), (37, 32),
            (40, 33), (42, 24), (41, 35), (41, 43), (37, 48), (37, 51)], "amber_light")
    s.poly([(36, 35), (40, 34), (40, 43), (36, 47), (30, 44), (30, 40)], "amber")
    s.rect((26, 51, 40, 58), "ink")
    s.rect((28, 53, 38, 56), "cyan")
    s.rect((28, 53, 30, 55), "cyan_light")
    s.line([(19, 20), (16, 17)], "amber")
    s.line([(44, 11), (47, 7)], "amber_light")
    s.star(55, 13, "lime_light", 1)
    return s.image


def unexpected_tail():
    s = Sprite()
    # A tidy decimal capsule has acquired an absurdly long articulated tail.
    s.line([(28, 35), (34, 44), (45, 48), (54, 44), (56, 35),
            (50, 28), (45, 20), (49, 12)], "ink", 6)
    s.line([(28, 35), (35, 43), (45, 46), (52, 42), (54, 35),
            (48, 28), (43, 20), (47, 12)], "amber_dark", 4)
    s.line([(30, 36), (35, 42), (45, 45), (51, 41), (53, 35)], "amber", 2)
    for x, y in ((34, 42), (43, 46), (52, 40), (50, 29), (44, 21), (47, 12)):
        s.rect((x-2, y-2, x+2, y+2), "ink")
        s.rect((x-1, y-1, x+1, y+1), "amber")
        s.rect((x-1, y-1, x, y), "amber_light")
    s.poly([(7, 24), (12, 18), (26, 18), (33, 24), (33, 35),
            (26, 41), (12, 41), (7, 35)], "ink")
    s.poly([(9, 25), (13, 20), (25, 20), (30, 25), (30, 34),
            (25, 38), (13, 38), (9, 34)], "steel_mid")
    s.line([(10, 25), (14, 21), (24, 21)], "steel_light", 2)
    s.rect((11, 25, 28, 34), "shadow")
    # Native custom pixel lettering: 0.3, then the unexpected terminal 4.
    s.rect((13, 27, 16, 32), "cyan_light")
    s.rect((14, 28, 15, 31), "shadow")
    s.rect((18, 32, 19, 33), "lime_light")
    s.line([(22, 27), (25, 27), (25, 29), (23, 30), (25, 31),
            (25, 32), (22, 32)], "cyan_light")
    s.poly([(47, 5), (57, 5), (60, 8), (60, 18), (57, 21), (47, 21),
            (44, 18), (44, 8)], "ink")
    s.rect((47, 7, 57, 18), "amber_dark")
    s.line([(48, 7), (56, 7)], "amber_light")
    s.numeral("4", 50, 9, "amber_light")
    s.line([(11, 47), (18, 48), (22, 46)], "cyan_dark")
    s.rect((8, 13, 9, 14), "cyan")
    s.star(26, 8, "lime_light", 1)
    s.rect((31, 53, 32, 54), "amber")
    return s.image


DRAWINGS = {
    "answer_found": answer_found,
    "six_seven": six_seven,
    "nice_number": nice_number,
    "result_found": result_found,
    "peer_review": peer_review,
    "bracket_architect": bracket_architect,
    "scientific_method": scientific_method,
    "touch_grass": touch_grass,
    "second_wind": second_wind,
    "alternate_routes": alternate_routes,
    "quiet_after_storm": quiet_after_storm,
    "paper_tiger": paper_tiger,
    "gaining_altitude": gaining_altitude,
    "mirror_room": mirror_room,
    "trouble_collector": trouble_collector,
    "unscathed": unscathed,
    "grand_scale": grand_scale,
    "last_pixel": last_pixel,
    "parallel_worlds": parallel_worlds,
    "time_loop": time_loop,
    "fourth_wall": fourth_wall,
    "unexpected_tail": unexpected_tail,
}


def contact_sheet(images, destination):
    # Both true-size and 2x views, on the application's dark surface.
    cell_w, cell_h = 224, 220
    columns = 4
    rows = (len(images) + columns - 1) // columns
    sheet = Image.new("RGB", (cell_w*columns, cell_h*rows), "#101016")
    draw = ImageDraw.Draw(sheet)
    font = ImageFont.load_default()
    for index, (name, image) in enumerate(images.items()):
        x = (index % columns)*cell_w
        y = (index // columns)*cell_h
        draw.text((x+12, y+10), name, fill="#d3dfcf", font=font)
        sheet.paste(image, (x+12, y+48), image)
        large = image.resize((128, 128), Image.Resampling.NEAREST)
        sheet.paste(large, (x+84, y+42), large)
        draw.text((x+18, y+183), "64px", fill="#86a6aa", font=font)
        draw.text((x+125, y+183), "128px / nearest", fill="#86a6aa", font=font)
    destination.parent.mkdir(parents=True, exist_ok=True)
    sheet.save(destination)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--contact-sheet", type=Path, help="Optional review image outside the repository")
    parser.add_argument("--libresprite", default="libresprite", help="LibreSprite binary for ASE export")
    args = parser.parse_args()
    source = Path(__file__).resolve().parent
    export = source.parents[1] / "web" / "public" / "achievements"
    binary = shutil.which(args.libresprite)
    if binary is None:
        parser.error("LibreSprite is required to export the editable ASE source files")
    export.mkdir(parents=True, exist_ok=True)
    images = {}
    for name, draw_sprite in DRAWINGS.items():
        image = draw_sprite()
        png = export / f"{name}.png"
        ase = source / f"{name}.ase"
        image.save(png, optimize=True)
        subprocess.run([binary, "--batch", str(png), "--save-as", str(ase)], check=True,
                       stdout=subprocess.DEVNULL)
        images[name] = image
        print(f"{png.relative_to(source.parents[1])} -> {ase.relative_to(source.parents[1])}")
    if args.contact_sheet:
        contact_sheet(images, args.contact_sheet)
        print(f"Contact sheet: {args.contact_sheet}")


if __name__ == "__main__":
    main()
