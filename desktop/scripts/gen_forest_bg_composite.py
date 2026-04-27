import math
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter


ROOT = Path(__file__).resolve().parents[1]
BG_OUTPUT = ROOT / "public" / "forest-batch-bg-v2.png"
SPRITE_OUTPUT = ROOT / "public" / "forest-firegrass-sprite-v2.png"
FRAME_COUNT = 8


def clamp(value, lower, upper):
    return max(lower, min(upper, value))


def draw_vertical_gradient(size, top_color, bottom_color):
    width, height = size
    image = Image.new("RGBA", size, (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)
    for y in range(height):
        t = y / max(height - 1, 1)
        color = tuple(
            int(top_color[index] * (1 - t) + bottom_color[index] * t)
            for index in range(4)
        )
        draw.line([(0, y), (width, y)], fill=color)
    return image


def add_forest_grade(base: Image.Image) -> Image.Image:
    canvas = base.convert("RGBA")
    w, h = canvas.size

    mist = Image.new("RGBA", canvas.size, (0, 0, 0, 0))
    d_mist = ImageDraw.Draw(mist)
    d_mist.ellipse((-160, 30, int(w * 0.44), int(h * 0.58)), fill=(132, 186, 147, 28))
    d_mist.ellipse((int(w * 0.58), 10, w + 150, int(h * 0.46)), fill=(247, 206, 129, 26))
    d_mist.ellipse((int(w * 0.24), int(h * 0.6), int(w * 0.8), h + 120), fill=(66, 120, 82, 36))
    mist = mist.filter(ImageFilter.GaussianBlur(44))
    canvas.alpha_composite(mist)

    glow = Image.new("RGBA", canvas.size, (0, 0, 0, 0))
    d_glow = ImageDraw.Draw(glow)
    d_glow.polygon(
        [
            (int(w * 0.47), h),
            (int(w * 0.53), h),
            (int(w * 0.62), int(h * 0.73)),
            (int(w * 0.58), int(h * 0.5)),
            (int(w * 0.63), int(h * 0.24)),
            (int(w * 0.55), 0),
            (int(w * 0.45), 0),
            (int(w * 0.37), int(h * 0.24)),
            (int(w * 0.42), int(h * 0.5)),
            (int(w * 0.38), int(h * 0.73)),
        ],
        fill=(255, 219, 154, 42),
    )
    glow = glow.filter(ImageFilter.GaussianBlur(32))
    canvas.alpha_composite(glow)

    grade = Image.new("RGBA", canvas.size, (10, 21, 15, 44))
    canvas = Image.blend(canvas, grade, 0.18)
    return canvas.convert("RGB")


def generate_background():
    width, height = 1360, 820
    print("Step 1: drawing forest backdrop...")

    canvas = draw_vertical_gradient(
        (width, height),
        (26, 51, 35, 255),
        (6, 15, 10, 255),
    )

    sky_glow = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    d_sky = ImageDraw.Draw(sky_glow)
    d_sky.ellipse((width * 0.24, -120, width * 0.76, height * 0.48), fill=(196, 227, 156, 72))
    d_sky.ellipse((width * 0.40, -10, width * 0.60, height * 0.34), fill=(255, 206, 114, 48))
    sky_glow = sky_glow.filter(ImageFilter.GaussianBlur(72))
    canvas.alpha_composite(sky_glow)

    mist = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    d_mist = ImageDraw.Draw(mist)
    d_mist.ellipse((-180, 40, width * 0.42, height * 0.62), fill=(142, 189, 151, 26))
    d_mist.ellipse((width * 0.58, 10, width + 160, height * 0.50), fill=(248, 210, 132, 18))
    d_mist.ellipse((width * 0.18, height * 0.56, width * 0.82, height + 160), fill=(62, 110, 75, 34))
    mist = mist.filter(ImageFilter.GaussianBlur(48))
    canvas.alpha_composite(mist)

    tree_layers = [
        ((19, 47, 28, 150), 0.62, 24, 0.14, 0.24, 6),
        ((15, 36, 22, 186), 0.73, 20, 0.12, 0.22, 4),
        ((11, 27, 17, 224), 0.84, 18, 0.10, 0.18, 2),
    ]
    for color, baseline, count, min_ratio, max_ratio, blur_radius in tree_layers:
        layer = Image.new("RGBA", (width, height), (0, 0, 0, 0))
        draw = ImageDraw.Draw(layer)
        for index in range(count):
            t = index / max(count - 1, 1)
            center_x = int(width * (0.04 + 0.92 * t) + math.sin(index * 1.83) * 20)
            canopy_w = int(width * (min_ratio + (max_ratio - min_ratio) * abs(math.sin(index * 1.27))))
            canopy_h = int(height * (0.15 + 0.12 * abs(math.cos(index * 1.41))))
            base_y = int(height * baseline + math.sin(index * 0.88) * 10)
            trunk_w = max(8, canopy_w // 10)
            draw.rectangle(
                (center_x - trunk_w // 2, base_y - canopy_h * 0.52, center_x + trunk_w // 2, base_y + canopy_h * 0.24),
                fill=(31, 24, 17, min(color[3] + 18, 255)),
            )
            draw.ellipse(
                (center_x - canopy_w, base_y - canopy_h, center_x + canopy_w, base_y + canopy_h * 0.18),
                fill=color,
            )
            draw.ellipse(
                (center_x - canopy_w * 0.66, base_y - canopy_h * 1.08, center_x + canopy_w * 0.66, base_y - canopy_h * 0.18),
                fill=(min(color[0] + 8, 255), min(color[1] + 10, 255), min(color[2] + 7, 255), color[3]),
            )
        layer = layer.filter(ImageFilter.GaussianBlur(blur_radius))
        canvas.alpha_composite(layer)

    path = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    d_path = ImageDraw.Draw(path)
    d_path.polygon(
        [
            (width * 0.44, height),
            (width * 0.56, height),
            (width * 0.62, height * 0.82),
            (width * 0.57, height * 0.56),
            (width * 0.64, height * 0.30),
            (width * 0.55, 0),
            (width * 0.45, 0),
            (width * 0.36, height * 0.30),
            (width * 0.43, height * 0.56),
            (width * 0.38, height * 0.82),
        ],
        fill=(255, 217, 146, 52),
    )
    path = path.filter(ImageFilter.GaussianBlur(28))
    canvas.alpha_composite(path)

    foreground = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    d_fg = ImageDraw.Draw(foreground)
    for index in range(26):
        t = index / 25
        left_x = int(width * (0.03 + t * 0.34))
        right_x = int(width * (0.97 - t * 0.34))
        bush_w = int(width * (0.026 + 0.014 * abs(math.sin(index * 1.4))))
        bush_h = int(height * (0.12 + 0.08 * abs(math.cos(index * 1.2))))
        for cx in (left_x, right_x):
            d_fg.ellipse(
                (cx - bush_w, height - bush_h, cx + bush_w, height + bush_h * 0.18),
                fill=(8, 23, 14, 234),
            )
    foreground = foreground.filter(ImageFilter.GaussianBlur(1.2))
    canvas.alpha_composite(foreground)

    final = add_forest_grade(canvas.convert("RGB"))
    BG_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    final.save(BG_OUTPUT)
    print(f"Saved background: {BG_OUTPUT}")


def draw_blade(draw, x, base_y, height, width, tilt, color):
    tip_x = x + tilt
    points = [
        (x, base_y),
        (x + width, base_y),
        (tip_x + width // 2, base_y - height),
    ]
    draw.polygon(points, fill=color)


def generate_firegrass_sprite():
    frame_w, frame_h = 280, 180
    sprite = Image.new("RGBA", (frame_w * FRAME_COUNT, frame_h), (0, 0, 0, 0))

    for frame in range(FRAME_COUNT):
        phase = frame / FRAME_COUNT
        img = Image.new("RGBA", (frame_w, frame_h), (0, 0, 0, 0))
        glow = Image.new("RGBA", (frame_w, frame_h), (0, 0, 0, 0))
        grass = Image.new("RGBA", (frame_w, frame_h), (0, 0, 0, 0))
        d_glow = ImageDraw.Draw(glow)
        d_grass = ImageDraw.Draw(grass)

        for index in range(22):
            t = index / 21
            x = int(26 + t * 228 + math.sin(phase * math.tau + index * 0.6) * 3)
            h = int(32 + (1 - abs(t - 0.5) * 1.35) * 56 + math.sin(phase * math.tau * 1.4 + index) * 7)
            w = 3 if index % 3 else 4
            tilt = int(math.sin(phase * math.tau * 1.2 + index * 0.45) * 12)
            color = (
                52 + (index % 4) * 8,
                101 + (index % 5) * 12,
                48 + (index % 3) * 6,
                210,
            )
            draw_blade(d_grass, x, frame_h - 12, h, w, tilt, color)

        ember_points = []
        for ember_index in range(5):
            et = ember_index / 4 if ember_index else 0
            ex = int(52 + et * 176 + math.sin(phase * math.tau + ember_index * 0.9) * 9)
            ey = int(frame_h - 46 - ember_index * 9 - math.cos(phase * math.tau * 1.3 + ember_index) * 6)
            radius = clamp(int(6 + math.sin(phase * math.tau + ember_index) * 2), 4, 8)
            ember_points.append((ex, ey, radius))

        for ex, ey, radius in ember_points:
            d_glow.ellipse((ex - radius * 4, ey - radius * 3, ex + radius * 4, ey + radius * 3), fill=(255, 176, 70, 18))
            d_glow.ellipse((ex - radius * 2, ey - radius * 2, ex + radius * 2, ey + radius * 2), fill=(255, 214, 122, 38))
            d_glow.ellipse((ex - radius, ey - radius, ex + radius, ey + radius), fill=(255, 244, 205, 160))

        for flicker_index in range(4):
            center_x = int(68 + flicker_index * 44 + math.sin(phase * math.tau * 1.5 + flicker_index) * 7)
            base_y = int(frame_h - 24 - flicker_index * 4)
            flame_h = clamp(int(24 + math.sin(phase * math.tau * 1.8 + flicker_index * 0.7) * 7), 16, 34)
            flame_w = clamp(int(14 + math.cos(phase * math.tau + flicker_index) * 3), 10, 18)
            d_glow.polygon(
                [
                    (center_x, base_y),
                    (center_x - flame_w // 2, base_y - flame_h // 2),
                    (center_x, base_y - flame_h),
                    (center_x + flame_w // 2, base_y - flame_h // 2),
                ],
                fill=(255, 166, 72, 90),
            )
            d_glow.ellipse(
                (
                    center_x - flame_w // 4,
                    base_y - flame_h + 6,
                    center_x + flame_w // 4,
                    base_y - flame_h // 2,
                ),
                fill=(255, 230, 174, 110),
            )

        glow = glow.filter(ImageFilter.GaussianBlur(7))

        floor_glow = Image.new("RGBA", (frame_w, frame_h), (0, 0, 0, 0))
        d_floor = ImageDraw.Draw(floor_glow)
        d_floor.ellipse((12, frame_h - 56, frame_w - 10, frame_h + 8), fill=(127, 255, 146, 36))
        floor_glow = floor_glow.filter(ImageFilter.GaussianBlur(12))

        img.alpha_composite(floor_glow)
        img.alpha_composite(glow)
        img.alpha_composite(grass)
        sprite.paste(img, (frame * frame_w, 0), img)

    SPRITE_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    sprite.save(SPRITE_OUTPUT)
    print(f"Saved firegrass sprite: {SPRITE_OUTPUT}")


def main():
    generate_background()
    print("Step 3: drawing animated firegrass frames...")
    generate_firegrass_sprite()


if __name__ == "__main__":
    main()
