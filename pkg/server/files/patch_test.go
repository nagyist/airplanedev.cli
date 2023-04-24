package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/airplanedev/cli/pkg/server/state"
	"github.com/stretchr/testify/require"
)

func TestPatch(t *testing.T) {
	require := require.New(t)

	// Create a temporary directory since we'll be performing changes to files.
	dir, err := os.MkdirTemp("", "cli_test")
	require.NoError(err)

	contents, err := os.ReadFile(filepath.Join("fixtures", "MyView.airplane.tsx"))
	require.NoError(err)

	filePath := filepath.Join(dir, "MyView.airplane.tsx")
	err = os.WriteFile(filePath, contents, 0644)
	require.NoError(err)

	patch := `--- MyView.airplane.tsx
+++ MyView.airplane.tsx
@@ -1,5 +1,6 @@
 import {
   Column,
+  Table,
   Stack,
   Text,
   Heading,
@@ -14,6 +15,10 @@
   const selectedCustomer = customersState.selectedRow;

   return (
     <Stack>
       <Heading>Customer overview</Heading>
       <Text>An example view that showcases customers and users.</Text>
+      <Table
+        columns={customersCols}
+        data={customersData}
+      />
     </Stack>
   );
 };
`
	err = Patch(&state.State{
		Dir: dir,
	}, patch)
	require.NoError(err)

	newContents, err := os.ReadFile(filePath)
	require.NoError(err)
	require.Equal(`import {
  Column,
  Table,
  Stack,
  Text,
  Heading,
  useComponentState,
} from "@airplane/views";
import airplane from "airplane";

// Put the main logic of the view here.
// Views documentation: https://docs.airplane.dev/views/getting-started
const ExampleView = () => {
  const customersState = useComponentState();
  const selectedCustomer = customersState.selectedRow;

  return (
    <Stack>
      <Heading>Customer overview</Heading>
      <Text>An example view that showcases customers and users.</Text>
      <Table
        columns={customersCols}
        data={customersData}
      />
    </Stack>
  );
};
`, string(newContents))
}

func TestPatchReplace(t *testing.T) {
	require := require.New(t)

	// Create a temporary directory since we'll be performing changes to files.
	dir, err := os.MkdirTemp("", "cli_test")
	require.NoError(err)

	contents, err := os.ReadFile(filepath.Join("fixtures", "Cats.airplane.tsx"))
	require.NoError(err)

	filePath := filepath.Join(dir, "Cats.airplane.tsx")
	err = os.WriteFile(filePath, contents, 0644)
	require.NoError(err)

	patch := `Index: Cats.airplane.tsx
===================================================================
--- Cats.airplane.tsx
+++ Cats.airplane.tsx
@@ -3,13 +3,13 @@
 const Demo = () => {
   return (
     <Stack spacing="lg">
       <Table
-        title="Cats"
+        title="Dogs"
         data={[
-          { name: "Xiaohuang", breed: "Abyssinian" },
-          { name: "Peaches", breed: "Birman" },
-          { name: "Baosky", breed: "British Shorthair" },
+          { name: "Buddy", breed: "Golden Retriever" },
+          { name: "Max", breed: "German Shepherd" },
+          { name: "Bella", breed: "Labrador Retriever" },
         ]}
       />
     </Stack>
   );
`
	err = Patch(&state.State{
		Dir: dir,
	}, patch)
	require.NoError(err)

	newContents, err := os.ReadFile(filePath)
	require.NoError(err)
	require.Equal(`import { Callout, Heading, Link, Stack, Text, Table } from "@airplane/views";
import airplane from "airplane";
const Demo = () => {
  return (
    <Stack spacing="lg">
      <Table
        title="Dogs"
        data={[
          { name: "Buddy", breed: "Golden Retriever" },
          { name: "Max", breed: "German Shepherd" },
          { name: "Bella", breed: "Labrador Retriever" },
        ]}
      />
    </Stack>
  );
};
export default airplane.view(
  {
    slug: "demo",
    name: "Demo",
  },
  Demo
);
`, string(newContents))
}

func TestPatchNewFile(t *testing.T) {
	require := require.New(t)

	// Create a temporary directory since we'll be performing changes to files.
	dir, err := os.MkdirTemp("", "cli_test")
	require.NoError(err)

	patch := `--- sum.py
+++ sum.py
@@ -0,0 +1,2 @@
+def sum(a, b):
+    return a + b
`
	err = Patch(&state.State{
		Dir: dir,
	}, patch)
	require.NoError(err)

	newContents, err := os.ReadFile(filepath.Join(dir, "sum.py"))
	require.NoError(err)
	require.Equal(`def sum(a, b):
    return a + b
`, string(newContents))
}
