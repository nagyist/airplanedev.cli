# Linked to https://app.airplane.dev/t/python_simple [do not edit this line]


def main(params):
    with open("preinstall.txt", "r") as f:
        preinstall = f.read().strip()
    with open("postinstall.txt", "r") as f:
        postinstall = f.read().strip()
    print(f"{preinstall=} {postinstall=}")
