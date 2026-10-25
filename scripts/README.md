# Scripts

This is a where you can find various helper scripts.
Of interest I'd recommend, if you use nushell, the [original todo.nu](./nushell/todo.nu).


## todo.nu

Before there was a `todone` cli and tui there was a todo.nu.
This was the quick and dirty task listing script whose logic was the precursor for the `todone`.
Better yet it is compatible with the todo.yml used by todone, so both can be used at the same time.
Also the todo.nu brings some functionality that is not supported by todone like importing tasks from ticketing systems.

### Usage

The todo script uses `nushell v0.116.x`.
First load the module:

> use nushell/todo.nu

Help can list you your options:

> help todo

To add a task use:

> todo add

To list tasks:

> todo list

To find tasks:

> todo fuzzy

To toggle tasks to done:

> todo done:

#### Importing tasks

The tasks are stored on a todo.yml file, so tasks are easy to import export, just copy or append the/to todo.yaml.
But todo.nu allows you to fetch information from ticket manger software and upsert it one of your todo.yml

_Note:_ Currently onle azure devops is supported.

To import from azure devops use:

> todo import ado -p <project_name> -o <org> --pat <your_pat> work_item_id_0 work_item_id_1 ...

or set the PAT as an env variable with

> todo set pat <pat>

or via your env.nu file (be careful not leak it)

The import workflow allows to:

> todo import ... work_item_id
> todone follow task_id

This will fetch the summary for the work item and return the todone task id, adding the work item url as the primary todone link.
This in turn allows you to open the work item on your default browser from the cli.
