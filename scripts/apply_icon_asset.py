from pathlib import Path

from PIL import Image


ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "Build" / "Gemini_Generated_Image_2nml9g2nml9g2nml.png"
PNG_TARGETS = [
    ROOT / "build" / "appicon.png",
    ROOT / "public" / "logo.png",
    ROOT / "src" / "assets" / "logo.png",
]
ICO_TARGET = ROOT / "build" / "windows" / "icon.ico"


def main() -> None:
    image = Image.open(SOURCE).convert("RGBA")
    image = image.resize((1024, 1024), Image.Resampling.LANCZOS)

    for target in PNG_TARGETS:
        target.parent.mkdir(parents=True, exist_ok=True)
        image.save(target)
        print(f"saved png: {target}")

    ICO_TARGET.parent.mkdir(parents=True, exist_ok=True)
    image.save(
        ICO_TARGET,
        format="ICO",
        sizes=[(256, 256), (128, 128), (96, 96), (64, 64), (48, 48), (32, 32), (16, 16)],
    )
    print(f"saved ico: {ICO_TARGET}")


if __name__ == "__main__":
    main()
