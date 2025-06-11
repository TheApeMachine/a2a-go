# Azure DevOps Tools

This package contains a set of high-level, focused tools for an AI agent to interact with Azure DevOps.

## Tools

The tools come in a few different categories that indicate certain parts of a standard workflow.

### Sprint Management

For sprint management, the model should be able to either provide a specific sprint to operate on, or it can use @CurrentIteration to operate on the current sprint.

- `get_sprints`: Get all sprints in Azure DevOps, useful for the model to understand which sprints exist, especially when creating new sprints.
- `create_sprint`: Create a new sprint in Azure DevOps.
- `sprint_items`: Get the items in a sprint in Azure DevOps.
- `sprint_overview`: Get the overview of a sprint in Azure DevOps.

### Work Item Management

To keep the amount of tools manageable, for the most part each tool will default to bulk operations. The model could just provide a list of one item to operate on, or it can provide a list of items to operate on.

We also want to maximize the amount of information that can be provided to the model to make decisions.

This includes returning related work items, as well as the ability to add related work items to a work item.

This also includes returning comments from work items, as well as the ability to add comments to work items.

- `create_work_items`: Create a new work item in Azure DevOps.
- `get_work_items`: Get the details of a work item in Azure DevOps.
- `update_work_items`: Update a work item in Azure DevOps. This should be capable of dealing with the full range of work item fields, including assignment, status, custom fields, sprint, relationships, comments, etc.

### Miscellaneous

- `search_work_items`: Search for work items in Azure DevOps by keywords, abstracting away the WIQL query.
- `execute_wiql`: Execute a WIQL query on Azure DevOps, returning the results.

### REVIEW

If you consider the tools you have available now for azure, assuming for the moment they would all be working, is there anything missing for you to effectively manage azure boards and work items?

Edit
Looking at the Azure DevOps tools available, you have a pretty comprehensive set for work item management. Let me analyze what's there and what might be missing for truly effective Azure Boards management:

Current Tool Coverage ✅
Core CRUD Operations:

✅ Create work items (azure_create_work_items)
✅ Read/Get work items (azure_get_work_items)
✅ Update work items (azure_update_work_items)
✅ Search/Query work items (azure_execute_wiql, azure_search_work_items)
Sprint Management:

✅ Get sprints (azure_get_sprints)
✅ Create sprints (azure_create_sprint)
✅ Sprint overview (azure_sprint_overview)
✅ Sprint work items (azure_sprint_items)

Notable Gaps for Advanced Management 🔍

1. Work Item Deletion & State Management:

No bulk state transitions (moving multiple items from "Doing" to "Done")

1. Advanced Relationship Management:

Can add/remove relations in updates, but no dedicated relationship visualization

No dependency mapping or critical path analysis
No bulk relationship operations

3. Team & Capacity Management:

No workload distribution analysis
No team velocity metrics

1. Board Configuration:

No board customization (columns, swimlanes, card fields)
No board rules management
No custom field definitions

5. Advanced Analytics & Reporting:

No burndown chart data
No cycle time analysis
No work item history/audit trails
No bulk export capabilities
6. Process Template Management:

No work item type customization
No workflow state customization
No field rule management
7. Integration & Automation:

No webhook management
No automated rule creation
No integration with external tools (GitHub, etc.)
Most Critical Missing Pieces for Your Use Case 🎯
Given your development background and need for out-of-the-box solutions, I'd prioritize these missing capabilities:

1. Bulk Operations Tool:

javascript
// Conceptual tool for bulk state changes
azure_bulk_update({
  query: "SELECT [System.Id] FROM WorkItems WHERE [System.State] = 'DOING'",
  updates: { state: "DONE", resolved_date: "today" }
})
2. Advanced Analytics Tool:

javascript
// Sprint metrics and team performance
azure_get_analytics({
  sprint_id: "current",
  metrics: ["velocity", "burndown", "cycle_time", "blocked_items"]
})
3. Work Item Templates:

javascript
// Standardized work item creation
azure_create_from_template({
  template: "bug_template",
  values: { severity: "high", component: "api" }
})
4. Dependency Analysis:

javascript
// Critical path and blocking analysis  
azure_analyze_dependencies({
  root_item_id: 1964,
  depth: 3,
  include_blocked: true
})
The current toolset covers about 70-80% of typical Azure Boards management needs. The gaps are mainly in advanced analytics, bulk operations, and process automation - areas where you'd typically want more sophisticated tooling for enterprise-level project management.