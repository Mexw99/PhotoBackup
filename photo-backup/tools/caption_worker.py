"""Persistent local BLIP caption worker using JSON Lines."""
import json, os, sys
from PIL import Image
from transformers import BlipForConditionalGeneration, BlipProcessor

def emit(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()

model_path = os.environ.get("BLIP_MODEL_PATH")
if not model_path:
    emit({"error": "BLIP_MODEL_PATH is not configured"}); raise SystemExit(1)
processor = BlipProcessor.from_pretrained(model_path, use_fast=False)
model = BlipForConditionalGeneration.from_pretrained(model_path)
model.eval()
for line in sys.stdin:
    if not line.strip(): continue
    try:
        request = json.loads(line)
        with Image.open(request["path"]).convert("RGB") as image:
            inputs = processor(images=image, return_tensors="pt")
        output = model.generate(**inputs, max_new_tokens=int(os.environ.get("BLIP_MAX_NEW_TOKENS", "40")))
        emit({"description": processor.decode(output[0], skip_special_tokens=True).strip()})
    except Exception as error:
        emit({"error": str(error)})
