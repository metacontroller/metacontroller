function(request) {
  // If the StatefulSet is updated to no longer match our decorator selector,
  // or if the StatefulSet is deleted, clean up any attachments we made.
  attachments: [],
  // Mark as finalized once we observe all Services are gone.
  // In v2 DecoratorController, the field is called 'attachments'.
  finalized: std.length(request.attachments['Service.v1']) == 0
}
