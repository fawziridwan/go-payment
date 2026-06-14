import shutil
import os

path = r'c:\code\go-payment\api'
if os.path.exists(path):
    shutil.rmtree(path)
    print(f"Deleted {path}")
else:
    print(f"{path} does not exist")
