# Overview

## Process
- ./src/ : directory for all implementation files (excluding main.go)
- ./main.go : dispatches to each following section in order
- pipeline.go / pipeline_async.go : multithreaded pipeline handling lexing, parsing and info gathering
- tokenizer.go : lexer, takes in a string and returns meaningful tokens
- parser.go : forms higher meaning nested constructs from token combinations, generates AST
- scope_info.go : generates mappings between constructs for easier information retrieval later
- link_checker.go : checks validity of names and generates mappings between names and constructs
- type_checker.go : verifies type assumptions and that types are correctly followed
- llvm_ir.go : 1 to 1 lowering of the AST to LLVM IR

## Syntax
- ./samples/ : sample files describing the syntax of the language
- ./SYNTAX.md : documentation of the syntax

## Standard library
- ./std/ : directory for all standard libraries

> Note:  
> file.mg displays usage of platform-dependent code based on compilation environment using the @platform compile-time macro.
