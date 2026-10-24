"""Persistent local YOLO worker used by the Wails Go process.

Protocol: one JSON request per line on stdin, one JSON response per line on
stdout. The model is loaded once so a large photo library does not pay Python
and model startup cost for every image.
"""

import json
import os
import sys

from ultralytics import YOLO


def emit(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()


base_dir = getattr(sys, "_MEIPASS", os.path.dirname(os.path.abspath(__file__)))
model_path = os.environ.get("YOLO_MODEL_PATH", os.path.join(base_dir, "yolo11n.pt"))
model = YOLO(model_path)

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        request = json.loads(line)
        image_path = request["path"]
        result = model.predict(
            source=image_path,
            imgsz=int(os.environ.get("YOLO_IMAGE_SIZE", "640")),
            conf=float(os.environ.get("YOLO_CONFIDENCE", "0.25")),
            device=os.environ.get("YOLO_DEVICE", "cpu"),
            verbose=False,
        )[0]
        names = result.names
        seen = set()
        tags = []
        for class_id in result.boxes.cls.tolist() if result.boxes is not None else []:
            name = str(names[int(class_id)]).strip()
            key = name.casefold()
            if name and key not in seen:
                seen.add(key)
                tags.append(name)
        emit({"tags": tags[:10]})
    except Exception as error:
        emit({"error": str(error)})
