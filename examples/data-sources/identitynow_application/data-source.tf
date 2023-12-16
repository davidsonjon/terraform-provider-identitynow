# Example Application data source
data "identitynow_application" "application" {
  id = "38383"
}

# Example Application data source by name
data "identitynow_application" "application_by_name" {
  name = "Example Application"
}
