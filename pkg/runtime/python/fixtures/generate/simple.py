import airplane


@airplane.task(
    slug="inline_python_full3",
    name="Inline python full3",
)
def inline_python_full3():
    data = [
        {"id": 1, "name": "Gabriel Davis", "role": "Dentist"},
        {"id": 2, "name": "Carolyn Garcia", "role": "Sales"},
        {"id": 3, "name": "Frances Hernandez", "role": "Astronaut"},
        {"id": 4, "name": "Melissa Rodriguez", "role": "Engineer"},
        {"id": 5, "name": "Jacob Hall", "role": "Engineer"},
        {"id": 6, "name": "Andrea Lopez", "role": "Astronaut"},
    ]

    # Sort the data in ascending order by name.
    data = sorted(data, key=lambda u: u["name"])

    # You can return data to show output to users.
    # Output documentation: https://docs.airplane.dev/tasks/output
    return data
