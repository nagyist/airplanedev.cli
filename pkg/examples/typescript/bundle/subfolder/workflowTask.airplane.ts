import airplane from "airplane";

export default airplane.task(
  {
    slug: "demo_upgrade_company",
    name: "Demo: Upgrade company",
    runtime: "workflow",
  },
  async () => {
    const data = [
      { id: 1, name: "Gabriel Davis", role: "Dentist" },
      { id: 2, name: "Carolyn Garcia", role: "Sales" },
      { id: 3, name: "Frances Hernandez", role: "Astronaut" },
      { id: 4, name: "Melissa Rodriguez", role: "Engineer" },
      { id: 5, name: "Jacob Hall", role: "Engineer" },
      { id: 6, name: "Andrea Lopez", role: "Astronaut" },
    ];

    // Sort the data in ascending order by name.
    data.sort((u1, u2) => {
      return u1.name.localeCompare(u2.name);
    });

    // You can return data to show output to users.
    // Output documentation: https://docs.airplane.dev/tasks/output
    return data;
  }
);
