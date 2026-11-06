# Homebase

A self-hosted application a household uses to run its shared work, schedules, and records.

## Household and people

**Household**:
The group of people sharing one Homebase; every record belongs to exactly one Household.
_Avoid_: Family, home, account, tenant

**Member**:
A person who belongs to a Household. Each Member has a role of either Owner or Member.
_Avoid_: User (when meaning a person in the household), resident

**Owner**:
The Member role that can manage the Household's Members and their roles.
_Avoid_: Admin

**User**:
The sign-in identity behind a Member. Use only when talking about authentication.
_Avoid_: Account

## Work

**Project**:
A larger piece of household work with its own status, priority, and due date that groups Project Tasks.
_Avoid_: Job

**Waiting**:
The Project status for work blocked on something outside the Household, such as a contractor, delivery, or reply.
_Avoid_: Blocked, on hold, paused

**Project Folder**:
A named, ordered grouping of Project Tasks within one Project.
_Avoid_: Section, group, category

**Task**:
A single assignable piece of work with a due date, priority, and status of open or done.
_Avoid_: To-do, chore, action item

**Standalone Task**:
A Task that does not belong to a Project.

**Project Task**:
A Task that belongs to a Project, optionally within a Project Folder.

**Generated Task**:
A Task created automatically by a Routine or a Maintenance Schedule when it comes due; completing it advances its source to the next due date.
_Avoid_: Recurring task, auto task

## Recurring obligations

**Cadence**:
How often a Routine or Maintenance Schedule recurs: daily, weekly, monthly, quarterly, or yearly.
_Avoid_: Frequency, interval, recurrence

**Routine**:
A household-level recurring obligation with a Cadence that produces Generated Tasks.
_Avoid_: Recurring task, chore schedule

**Maintenance Schedule**:
A recurring obligation with a Cadence attached to one Asset that produces Generated Tasks. An Asset may have many.
_Avoid_: Maintenance item, service interval

## Calendar

**Event**:
Anything that appears on the calendar at a date or time: an Appointment, a Task due date, a Project due date, or a Routine due date.
_Avoid_: Calendar entry, item

**Appointment**:
An Event the Household schedules directly, with a start, end, and location, such as a doctor's visit.
_Avoid_: Meeting, booking

**Attendee**:
A Member an Appointment is for. An Appointment with no Attendees is for the whole Household.
_Avoid_: Participant, guest, invitee

**Calendar Connection**:
One phone or calendar app a Member has connected to Homebase so it shows, and can change, the Events and Tasks that concern that Member. A Member may have many and can disconnect each one; all of them end when the Member is removed from the Household. Changes made on the device follow the same rules as changes made in Homebase, so a delete on the device is an Archive.
_Avoid_: Calendar Feed, calendar sync, calendar account, subscription

## Records

**Asset**:
A physical thing the Household owns and tracks, with purchase, warranty, and Maintenance Schedule details.
_Avoid_: Item, possession, equipment

**Contact**:
A person or business the Household deals with, reusable across Projects, Tasks, and Assets.
_Avoid_: Vendor, provider, person

**Document**:
An uploaded file kept as a reusable household record that can be related to Projects, Tasks, and Assets.
_Avoid_: File, attachment, upload

**Related Item**:
A connection between a reusable record (Document, Contact, or Asset) and a Project, Task, or Asset it pertains to. The record is shared, never copied.
_Avoid_: Link, attachment, association

**List**:
A shared household list, such as groceries or packing, made up of List Items.
_Avoid_: Household list, checklist, to-do list

**List Item**:
One entry on a List that can be assigned, given a due date, and completed. It is not a Task.
_Avoid_: Task, entry

**Archive**:
The only way a household record is removed. Archived records are hidden from active views and can be recovered. Hard deletion requires an explicit, documented exemption.
_Avoid_: Delete, remove, trash

## Notifications

**Notification**:
A message to one Member about something that happened or is coming due. It appears in the in-app bell and on the Member's devices. A Notification is not a household record, so the Archive rule does not apply: read Notifications expire.
_Avoid_: Alert, notice, message

**Assignment Notification**:
A Notification telling a Member that a Task, List Item, Routine, or Appointment has become theirs, whether another Member assigned it or a Routine or Maintenance Schedule generated it. It is never sent when you assign something to yourself.

**Reminder**:
A Notification that something assigned to a Member is coming due or is overdue. It is sent whoever made the assignment. A Task gets Due Soon, Due Now, and Overdue Reminders. An Appointment gets Due Soon, an hour-before Reminder, and Due Now.
_Avoid_: Alert, nudge

**Due Soon**:
The Reminder sent 24 hours before something is due. For a Task due on a date with no time of day, it is sent at 08:00 the day before.

**Due Now**:
The Reminder sent at the due date and time, or when an Appointment starts. For a Task due on a date with no time of day, it is sent at 08:00 on that date.

**Overdue**:
The Reminder sent the morning after the due date if the Task is still open.
_Avoid_: Late, past due

**Unread**:
A Notification the Member has not yet opened. The count of Unread Notifications is the badge on the app icon and the bell.

**Push**:
Delivery of a Notification to a Member's device while the app is closed.
_Avoid_: Push notification (as a noun for the message itself)

**Push Subscription**:
One browser or installed app that a Member has enabled to receive Push. A Member may have many.
_Avoid_: Device, registration, endpoint

## Interface

**Module**:
One area of the application, such as Projects, Assets, or Documents.
_Avoid_: Section, page, app

**Dashboard Tile**:
A movable, removable block on a Member's dashboard.
_Avoid_: Module, widget, card
