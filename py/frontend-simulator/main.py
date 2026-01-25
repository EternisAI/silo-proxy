import httpx
from fastapi import FastAPI, Request, Form
from fastapi.responses import HTMLResponse
from fastapi.templating import Jinja2Templates
from pathlib import Path
import json

app = FastAPI(title="Frontend Simulator")

templates_dir = Path(__file__).parent / "templates"
templates_dir.mkdir(exist_ok=True)
templates = Jinja2Templates(directory=str(templates_dir))

PROXY_URL = "http://localhost:8080"

@app.get("/", response_class=HTMLResponse)
async def index(request: Request):
    return templates.TemplateResponse("index.html", {"request": request})

@app.post("/api/proxy-request")
async def proxy_request(
    agent_id: str = Form(...),
    method: str = Form(...),
    path: str = Form(...),
    body: str = Form(""),
):
    url = f"{PROXY_URL}/proxy/{agent_id}{path}"

    result = {
        "request": {
            "url": url,
            "method": method,
            "path": path,
            "agent_id": agent_id,
        },
        "response": None,
        "error": None,
    }

    try:
        async with httpx.AsyncClient(timeout=30.0) as client:
            if method == "GET":
                response = await client.get(url)
            elif method == "POST":
                headers = {"Content-Type": "application/json"}
                response = await client.post(url, content=body, headers=headers)
            elif method == "PUT":
                headers = {"Content-Type": "application/json"}
                response = await client.put(url, content=body, headers=headers)
            elif method == "DELETE":
                response = await client.delete(url)
            else:
                raise ValueError(f"Unsupported method: {method}")

            try:
                response_data = response.json()
            except:
                response_data = response.text

            result["response"] = {
                "status_code": response.status_code,
                "headers": dict(response.headers),
                "body": response_data,
                "elapsed_ms": response.elapsed.total_seconds() * 1000,
            }
    except Exception as e:
        result["error"] = str(e)

    return result

@app.get("/health")
async def health():
    return {"status": "ok"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=5000)
