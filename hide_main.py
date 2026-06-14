import os

old_file = r'c:\code\go-payment\api\main.go'
new_file = r'c:\code\go-payment\api\main.go.bak'

if os.path.exists(old_file):
    if os.path.exists(new_file):
        os.remove(new_file)
    os.rename(old_file, new_file)
    print(f"Renamed {old_file} to {new_file}")
else:
    print(f"{old_file} does not exist")
