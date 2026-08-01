# A simple event-triggered workflow that sends an email whenever an
# identity's manager attribute changes. "definition" and "trigger.attributes"
# are raw JSON strings - their shapes depend entirely on the step "type"s
# used in "definition.steps" and the trigger's own "type", respectively (see
# the resource documentation for links to SailPoint's Workflows guide).
resource "identitynow_workflow_v1" "send_email_on_manager_change" {
  name        = "Send Email on Manager Change"
  description = "Send an email to the identity whose manager attribute changed."
  enabled     = false # Workflows cannot be created in an enabled state.

  owner = {
    type = "IDENTITY"
    id   = "2c91808568c529c60168cca6f90c1313"
  }

  trigger = {
    type = "EVENT"
    attributes = jsonencode({
      id     = "idn:identity-attributes-changed"
      filter = "$.changes[?(@.attribute == 'manager')]"
    })
  }

  definition = jsonencode({
    start = "Send Email Test"
    steps = {
      "Send Email" = {
        actionId = "sp:send-email"
        attributes = {
          body            = "This is a test"
          from            = "sailpoint@sailpoint.com"
          "recipientId.$" = "$.identity.id"
          subject         = "test"
        }
        nextStep     = "success"
        selectResult = null
        type         = "action"
      }
      success = {
        type = "success"
      }
    }
  })
}

# A scheduled-trigger workflow example - "trigger.attributes" shape differs
# entirely (frequency/cronString/timeZone instead of id/filter).
resource "identitynow_workflow_v1" "nightly_report" {
  name        = "Nightly Report"
  description = "Run a scheduled report every night."
  enabled     = false

  owner = {
    type = "IDENTITY"
    id   = "2c91808568c529c60168cca6f90c1313"
  }

  trigger = {
    type = "SCHEDULED"
    attributes = jsonencode({
      cronString = "0 0 * * *"
    })
  }

  definition = jsonencode({
    start = "success"
    steps = {
      success = {
        type = "success"
      }
    }
  })
}
