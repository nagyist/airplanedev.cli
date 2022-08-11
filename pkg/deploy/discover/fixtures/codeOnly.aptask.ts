// Mock what the airplane sdk wrappers do
export const collatz = {
    __airplane: {
        config: {
            slug: "collatz",
            name: "Collatz Conjecture Step",
            parameters: {
                num: {name: "Num", type: "integer"}
            },
        },
    },
}
