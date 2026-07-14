FROM python:3.11-slim

WORKDIR /app

# libgomp1 is required by some pythainlp tokenizer backends; fonts-thai-tlwg
# provides a Thai font wordcloud.png can use to render glyphs correctly.
RUN apt-get update \
    && apt-get install -y --no-install-recommends libgomp1 fonts-thai-tlwg \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py ./

ENV TOP_N_WORDS=80
ENV THAI_FONT_PATH=/usr/share/fonts/truetype/tlwg/Garuda.ttf
ENV PYTHONUNBUFFERED=1

EXPOSE 8000

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
