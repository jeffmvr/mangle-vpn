from rest_framework import permissions


class UserPermission(permissions.BasePermission):
    def has_permission(self, request, view):
        """
        Returns True if the user is authenticated.
        :return: bool
        """
        return request.user.is_active

    def has_object_permission(self, request, view, obj):
        """
        Returns whether current user owns the given object.
        :return: bool
        """
        if hasattr(obj, "user"):
            return obj.user == request.user
        return False
