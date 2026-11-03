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

## Interface

**Module**:
One area of the application, such as Projects, Assets, or Documents.
_Avoid_: Section, page, app

**Dashboard Tile**:
A movable, removable block on a Member's dashboard.
_Avoid_: Module, widget, card
