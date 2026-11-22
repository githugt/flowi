// The group Compare environments shows. It is shared so that the Manage environments screen can open the comparison
// for a group, and so that the choice stays when the tab is closed and opened again. null: nothing chosen yet, and the
// screen picks one (see pickGroup in ./envcompare).

export const compareChoice = $state<{ group: string | null }>({ group: null })
