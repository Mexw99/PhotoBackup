"""Persistent local BLIP caption worker using JSON Lines."""
import json, os, sys
from PIL import Image
from transformers import AutoModelForSeq2SeqLM, AutoTokenizer
from transformers import BlipForConditionalGeneration, BlipProcessor

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

def emit(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()

model_path = os.environ.get("BLIP_MODEL_PATH")
if not model_path:
    emit({"error": "BLIP_MODEL_PATH is not configured"}); raise SystemExit(1)
processor = BlipProcessor.from_pretrained(model_path, use_fast=False)
model = BlipForConditionalGeneration.from_pretrained(model_path)
model.eval()
translator_path = os.environ.get(
    "BLIP_TRANSLATOR_MODEL_PATH",
    os.path.join(os.path.dirname(model_path), "..", "opus-mt-en-mul"),
)
translator = None
translator_tokenizer = None
if os.path.isdir(translator_path):
    translator_tokenizer = AutoTokenizer.from_pretrained(translator_path)
    translator = AutoModelForSeq2SeqLM.from_pretrained(translator_path)
    translator.eval()


def translate_to_thai(text):
    if translator is None or not text:
        return text
    inputs = translator_tokenizer(">>tha<<" + text, return_tensors="pt")
    output = translator.generate(**inputs, max_new_tokens=40)
    return translator_tokenizer.decode(output[0], skip_special_tokens=True).strip()


for line in sys.stdin:
    if not line.strip(): continue
    try:
        request = json.loads(line)
        with Image.open(request["path"]).convert("RGB") as image:
            inputs = processor(images=image, return_tensors="pt")
        output = model.generate(**inputs, max_new_tokens=int(os.environ.get("BLIP_MAX_NEW_TOKENS", "40")))
        caption = processor.decode(output[0], skip_special_tokens=True).strip()
        emit({"description": translate_to_thai(caption)})
    except Exception as error:
        emit({"error": str(error)})
