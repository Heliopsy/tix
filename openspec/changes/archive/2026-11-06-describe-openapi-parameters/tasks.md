## 1. Generator

- [x] 1.1 Query parameters take their field's doc comment, embedded page fields included
- [x] 1.2 Path parameters take their description from `pathParamDocs`, per collection where a name is shared
- [x] 1.3 Prefix families are described from the doc comment of the constant that spells the prefix

## 2. Comments

- [x] 2.1 Query struct fields state format, defaults and allowed values

## 3. Guards

- [x] 3.1 `TestEveryParameterIsDescribed` fails on an undescribed parameter or prefix
