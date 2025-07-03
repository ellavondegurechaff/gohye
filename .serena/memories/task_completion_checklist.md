# Task Completion Checklist

When completing a task in the GoHYE project, ensure:

## Before Committing
1. **Code Quality**
   - Run `go fmt ./...` to format code
   - Run `golangci-lint run` to check for linting issues
   - Run `go vet ./...` for static analysis
   - Ensure no unused imports or variables

2. **Error Handling**
   - All errors are properly handled
   - Error messages are descriptive
   - Appropriate logging is in place

3. **Database Changes**
   - Schema changes are documented
   - Indexes are added for frequently queried fields
   - Repository methods follow established patterns

4. **Command Integration**
   - New commands are added to appropriate Commands slice
   - Handlers are properly wrapped with logging
   - Component handlers are registered if needed

5. **Testing**
   - Test new features through Discord interactions
   - Verify error cases are handled gracefully
   - Check for race conditions in concurrent operations

## Documentation
- Update CLAUDE.md if adding major features
- Add comments for complex logic
- Update command descriptions for clarity

## Performance
- Check for N+1 query problems
- Use connection pooling appropriately
- Implement caching where beneficial
- Use context for proper cancellation