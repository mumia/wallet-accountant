@Ignore
Feature: Tag category create helper
  Background:
    * url baseUrl.api
    * configure logPrettyRequest = true
    * configure logPrettyResponse = true
    * def sleep = function(millis){ java.lang.Thread.sleep(millis) }

  Scenario: Add tags to use on other features
    Given path "/tag-category"
    And request
    """
    {
      "categoryName": "#(category)",
      "categoryNotes": "",
      "tagName": "#(tag)",
      "tagNotes": ""
    }
    """
    When method post
    Then status 201
    And match response == read("tag/response-add-tag-to-new-category.json")
    * sleep(1000)