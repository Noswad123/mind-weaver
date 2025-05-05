import os
from dotenv import load_dotenv

load_dotenv()

DB_PATH = os.getenv("DB_PATH", "./")
NOTES_DIR = os.getenv("NOTES_DIR", "./")
NODE_RADIUS = 10
