#!/usr/bin/env python3
"""Draw the eight original 64px achievement sprites and export LibreSprite sources.

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


DRAWINGS = {
    "answer_found": answer_found,
    "six_seven": six_seven,
    "nice_number": nice_number,
    "result_found": result_found,
    "peer_review": peer_review,
    "bracket_architect": bracket_architect,
    "scientific_method": scientific_method,
    "touch_grass": touch_grass,
}


def contact_sheet(images, destination):
    # Both true-size and 2x views, on the application's dark surface.
    cell_w, cell_h = 224, 220
    sheet = Image.new("RGB", (cell_w*4, cell_h*2), "#101016")
    draw = ImageDraw.Draw(sheet)
    font = ImageFont.load_default()
    for index, (name, image) in enumerate(images.items()):
        x = (index % 4)*cell_w
        y = (index // 4)*cell_h
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
