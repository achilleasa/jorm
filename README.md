# jorm

Jorm is one of my research side-projects whose goal is to identify practical
approaches for decoupling [Juju](https://github.com/juju/juju)'s business logic
from the persistence layer (colloquially referred to as "state") with the ultimate
goal being to allow Juju to use multiple storage backends which the user can
select when bootstrapping their controllers based on their particular use-case
requirements.

Any solution that fits the above profile must satisfy the following requirements:
- It must lend itself to being progressively deployed. In other words, it 
  should be compatible with the current backend (mongo with mgo/txn client-/server-side txns)
  implementation, enable us to develop new features using it and at the same 
  time allow us to gradually port existing business logic to the new approach.
- It must allow us to **unit** test the business logic layer in complete isolation.
- It must provide a transactional context for implementing the business logic.

## How does it work

Jorm is a simplified ORM-like layer that is designed to work with the common
data access patterns used by Juju. It parses a set of model schemas (defined
as plain Go structs) and leverages code generation (via Go templates) to produce:
- A set of models with getters/setters and internal state tracking (version, per-field mutations, whether the model is persisted or not, whether the model is flagged for deletion etc.)
- An interface (per-model accessor) for retrieving a model by its PK, getting the model count, iterating (iterators also get their own interface) models based on a field filter or simply iterating all models.
- An interface (ModelNamespace) which embeds the above accessor interfaces and limits lookups to a particular **Juju model-uuid** while also serving as the logical transaction context for the business logic.
- A Store interface.
- A common store validation suite which ensures that all store implementations behave in exactly the same way. The test suite covers cases such as:
    - CRUD operations for each model (also ensuring that nested types are marshaled/unmarshaled appropriately).
    - Use of iterators (including detection of iterator leaks: iterators must always be closed).
    - Ensuring that version-mismatches (e.g. another thread already modified/deleted a model we are currently trying to change) when attempting committing changes are correctly detected and cause the transaction to be rolled back.
    - Ensuring that ModelNamespace implementations provide read-your-write semantics.
- Multiple store implementations (in-memory, sqlite and mongo) as well as a test
  suite for each one that hooks up each backend to the common store validation suite.

The code generator lives in the `generator` package and both its input (the schema package)
and its various outputs can be configured via flags as shown below:

```console
 generator -h
Usage of generator:
  -model-pkg string
        the package where the generated models will be stored (default "github.com/achilleasa/jorm/model")
  -schema-pkg string
        the package containing the model schema definitions (default "github.com/achilleasa/jorm/schema")
  -store-backend-pkg string
        the package where the generated store backends will be stored (default "github.com/achilleasa/jorm/store/backend")
  -store-pkg string
        the package where the generated store interfaces be stored (default "github.com/achilleasa/jorm/store")
  -store-test-pkg string
        the package where the generated store test suite will be stored (default "github.com/achilleasa/jorm/store/storetest")
```

## Defining schemas

The model schema parser scans all files in the specified schema package (`--schema-pkg`)
and processes any exported struct definitions looking for `schema` tags. The
format of a schema tag is: `schema:$backend_field_name,attributes...`. 

The `backend_field_name` placeholder specifies the preferred field name that
should be used by the store backend (e.g. a column name for sqlite, a doc field
for mongo, etc.). If omitted (or no schema tag is specified), a suitable
(lower) camel-cased name will be generated based on the Go field name.

The **optional** attribute list may contain any of the following attributes:
- `pk`. Specifies a field as the PK for the model.
- `find_by`. Specifies that we can lookup instances (i.e. queries return iterators) of this model filtering by this field.

The schema parser classifies structs into primary models or secondary/embeddable
models based on whether they specify a PK field or not. In addition, any field
that is either a pointer, a slice or a map is flagged as _nullable_. 

When parsing schema definitions, the parser implementation uses the `go/build`
tool-chain to compile the schema package and extract information about not only
the fully qualified type of each struct field but also the imports (if any)
needed to reference it from other models. 

In addition, the parser walks the type information tree to obtain the actual
type of each field. So for example, a field of type `state.Life`, gets a fully
qualified type of `github.com/juju/juju/state.Life` (with
`github.com/juju/juju/state` being the import dependency) and a resolved type
of `uint8` (`Life` is an alias to a `uint8`).  The sqlite backend generator
evaluates resolved types when mapping fields to a suitable SQL type as well 
as for deciding when a (non-scalar) field value must be serialized or not.

## Backend implementations

The repo includes templates for generating an in-memory, an sqlite and mongo (with selectable client- or server-side txns)
backend as well as the appropriate test-suites for hooking them up to the 
common store validation suite.


## More information and example output

You can find a more detailed description of the generated artifacts and how 
they can be used to construct a business logic driven operation in the 
[accompanying](https://discourse.charmhub.io/t/a-proposal-for-decoupling-juju-business-logic-from-the-persistence-layer-part-2/4609) discourse post.

Finally, the HEAD commit of the
[example](https://github.com/achilleasa/jorm/tree/example) branch includes a
schema sample as well as the full set of files that were produced by the Jorm
generator.
