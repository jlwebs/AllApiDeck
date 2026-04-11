import os
import random
from pathlib import Path

import torch
from diffusers import DiffusionPipeline
from PIL import Image, ImageFilter


ROOT = Path(__file__).resolve().parents[1]
PNG_TARGETS = [
    ROOT / "build" / "appicon.png",
    ROOT / "public" / "logo.png",
    ROOT / "src" / "assets" / "logo.png",
]
ICO_TARGET = ROOT / "build" / "windows" / "icon.ico"
RAW_OUTPUT = ROOT / "build" / "ai-slime-icon-raw.png"
FINAL_OUTPUT = ROOT / "build" / "ai-slime-icon-final.png"
HF_CACHE = r"F:\huggingface\cache"
MODEL_SNAPSHOT = (
    Path(HF_CACHE)
    / "hub"
    / "models--stabilityai--stable-diffusion-xl-base-1.0"
    / "snapshots"
    / "462165984030d82259a11f4367a4eed129e94a7b"
)
CANVAS_SIZE = 1024
INFERENCE_STEPS = 30
GUIDANCE_SCALE = 6.5

PROMPT = (
    "a cute green slime mascot icon, kawaii chibi style, soft spring fantasy illustration, "
    "round jelly body, tiny leaf on top, sparkling eyes, gentle blush, centered portrait, "
    "clean mint badge background, polished game launcher app icon, simple composition, "
    "high contrast readable silhouette, premium mascot design"
)

NEGATIVE_PROMPT = (
    "pixel art, lowres, blurry, horror, realistic photo, extra characters, text, watermark, "
    "busy background, complex scenery, deformed face, cropped head, frame, UI, border"
)


def build_pipeline():
    if os.path.isdir(HF_CACHE):
        os.environ["HF_HOME"] = HF_CACHE
        os.environ["HF_HUB_DISABLE_SYMLINKS"] = "1"

    pipe = DiffusionPipeline.from_pretrained(
        str(MODEL_SNAPSHOT),
        torch_dtype=torch.float16,
        variant="fp16",
        use_safetensors=True,
        local_files_only=True,
    )
    pipe.to("cuda")
    pipe.set_progress_bar_config(disable=False)
    return pipe


def soften_icon(image: Image.Image) -> Image.Image:
    image = image.convert("RGBA").resize((CANVAS_SIZE, CANVAS_SIZE), Image.Resampling.LANCZOS)

    # Keep the AI composition, only add a slight soft-focus polish for icon usage.
    glow = image.filter(ImageFilter.GaussianBlur(14))
    glow.putalpha(72)
    polished = Image.new("RGBA", image.size, (0, 0, 0, 0))
    polished.alpha_composite(glow)
    polished.alpha_composite(image)
    return polished


def main():
    if not torch.cuda.is_available():
        raise RuntimeError("CUDA is not available; this generator is intended to run on GPU.")

    pipe = build_pipeline()

    seed = random.randint(1, 999999)
    print(f"Generating cute slime icon with seed={seed}")
    generator = torch.Generator(device="cuda").manual_seed(seed)
    result = pipe(
        prompt=PROMPT,
        negative_prompt=NEGATIVE_PROMPT,
        num_inference_steps=INFERENCE_STEPS,
        guidance_scale=GUIDANCE_SCALE,
        height=CANVAS_SIZE,
        width=CANVAS_SIZE,
        generator=generator,
    )
    image = result.images[0]

    RAW_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    image.save(RAW_OUTPUT)
    print(f"saved raw: {RAW_OUTPUT}")

    final_image = soften_icon(image)
    FINAL_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    final_image.save(FINAL_OUTPUT)
    print(f"saved final preview: {FINAL_OUTPUT}")

    for path in PNG_TARGETS:
        path.parent.mkdir(parents=True, exist_ok=True)
        final_image.save(path)
        print(f"saved png: {path}")

    ICO_TARGET.parent.mkdir(parents=True, exist_ok=True)
    final_image.save(
        ICO_TARGET,
        format="ICO",
        sizes=[(256, 256), (128, 128), (96, 96), (64, 64), (48, 48), (32, 32), (16, 16)],
    )
    print(f"saved ico: {ICO_TARGET}")


if __name__ == "__main__":
    main()
