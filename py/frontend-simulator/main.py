from fastapi import FastAPI, Request
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.templating import Jinja2Templates
from pathlib import Path
from datetime import datetime

app = FastAPI(title="Frontend Simulator")

templates_dir = Path(__file__).parent / "templates"
templates_dir.mkdir(exist_ok=True)
templates = Jinja2Templates(directory=str(templates_dir))

@app.get("/", response_class=HTMLResponse)
async def home(request: Request):
    return templates.TemplateResponse("index.html", {
        "request": request,
        "timestamp": datetime.now().isoformat()
    })

@app.get("/api/status")
async def status():
    return {
        "status": "running",
        "service": "frontend-simulator",
        "timestamp": datetime.now().isoformat(),
        "message": "Connected via silo-proxy!"
    }

@app.post("/api/data")
async def create_data(request: Request):
    try:
        body = await request.json()
        return {
            "success": True,
            "received": body,
            "timestamp": datetime.now().isoformat()
        }
    except:
        return JSONResponse(
            status_code=400,
            content={"error": "Invalid JSON"}
        )

@app.get("/health")
async def health():
    return {"status": "ok"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=3000)
