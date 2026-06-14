import shutil
import os

files_to_delete = [
    r'c:\code\go-payment\internal',
    r'c:\code\go-payment\main.go',
    r'c:\code\go-payment\api\main.go'
]

for path in files_to_delete:
    try:
        if os.path.isfile(path):
            os.remove(path)
            print(f"Deleted file: {path}")
        elif os.path.isdir(path):
            shutil.rmtree(path)
            print(f"Deleted directory: {path}")
        else:
            print(f"Path not found: {path}")
    except Exception as e:
        print(f"Error deleting {path}: {e}")
