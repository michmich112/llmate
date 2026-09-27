Feature: Model alias management

  Scenario: Model alias create, edit, delete end-to-end
    Given I am authenticated
    And I create a provider named "E2E Ollama" with base URL "http://127.0.0.1:11434"
    And I register model "llama3" on that provider
    And I register model "claude-3" on that provider
    When I visit the route "/dashboard/models"
    Then I see the "Add Alias" button
    And I take a screenshot named "01-models-initial"
    When I open the alias dialog
    And I fill the alias input with "gpt-4"
    And I select the provider "E2E Ollama"
    And I wait for model "claude-3" options
    And I select model "llama3"
    And I click the "Create Alias" button
    Then I see the alias "gpt-4" in the table
    And I take a screenshot named "03-after-add"
    When I edit the alias "gpt-4" to name "claude" and model "claude-3"
    Then I see the alias "claude" in the table
    And the alias "claude" points to model "claude-3"
    And I take a screenshot named "04-after-edit"
    And the alias "claude" is persisted with model "claude-3"
    When I delete the alias "claude"
    Then no alias named "claude" exists
    And I take a screenshot named "05-after-delete"
