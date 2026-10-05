// The reference renderings of the visual parity suite (REQ-REG-08). Each
// entry mirrors the canonical fixture body of one registry item with the
// pinned shadcn/ui components. The parity spec diffs the pixels of both.
//
// The second parameter of each entry, when used, opens overlays so the
// spec can capture them.
import { useEffect, useRef, type ReactNode } from "react"
import { XIcon } from "lucide-react"
import { cn } from "@/lib/utils"
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/ui/accordion"
import { Alert, AlertDescription, AlertTitle } from "@/ui/alert"
import { AspectRatio } from "@/ui/aspect-ratio"
import { Avatar, AvatarFallback } from "@/ui/avatar"
import { Badge } from "@/ui/badge"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/ui/breadcrumb"
import { Button, buttonVariants } from "@/ui/button"
import { ButtonGroup } from "@/ui/button-group"
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/ui/card"
import { Checkbox } from "@/ui/checkbox"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/ui/collapsible"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/ui/empty"
import {
  Field,
  FieldDescription,
  FieldLabel,
} from "@/ui/field"
import { Input } from "@/ui/input"
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  InputGroupText,
} from "@/ui/input-group"
import { Item, ItemActions, ItemContent, ItemDescription, ItemMedia, ItemTitle } from "@/ui/item"
import { Kbd } from "@/ui/kbd"
import { Label } from "@/ui/label"
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/ui/pagination"
import { Progress } from "@/ui/progress"
import { RadioGroup, RadioGroupItem } from "@/ui/radio-group"
import { Separator } from "@/ui/separator"
import { Skeleton } from "@/ui/skeleton"
import { Spinner } from "@/ui/spinner"
import { Switch } from "@/ui/switch"
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/ui/table"
import { Textarea } from "@/ui/textarea"
import { Toggle } from "@/ui/toggle"
import { ToggleGroup, ToggleGroupItem } from "@/ui/toggle-group"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/ui/alert-dialog"
import {
  ContextMenu,
  ContextMenuCheckboxItem,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuRadioGroup,
  ContextMenuRadioItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
} from "@/ui/context-menu"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/ui/dialog"
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/ui/drawer"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from "@/ui/dropdown-menu"
import { HoverCard, HoverCardContent, HoverCardTrigger } from "@/ui/hover-card"
import {
  Menubar,
  MenubarCheckboxItem,
  MenubarContent,
  MenubarItem,
  MenubarLabel,
  MenubarMenu,
  MenubarRadioGroup,
  MenubarRadioItem,
  MenubarSeparator,
  MenubarShortcut,
  MenubarTrigger,
} from "@/ui/menubar"
import {
  NavigationMenu,
  NavigationMenuContent,
  NavigationMenuItem,
  NavigationMenuLink,
  NavigationMenuList,
  NavigationMenuTrigger,
  navigationMenuTriggerStyle,
} from "@/ui/navigation-menu"
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
} from "@/ui/popover"
import { Select, SelectTrigger, SelectValue } from "@/ui/select"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/ui/sheet"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/ui/tooltip"

// OpenContextMenu opens the pinned context menu the way a user does: the
// menu has no open prop, so the trigger receives the contextmenu event.
function OpenContextMenu({ open, children }: { open: boolean; children: ReactNode }) {
  const trigger = useRef<HTMLSpanElement>(null)
  useEffect(() => {
    if (!open) return
    trigger.current?.dispatchEvent(
      new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 16, clientY: 16 }),
    )
  }, [open])
  return (
    <ContextMenu>
      <ContextMenuTrigger ref={trigger}>Right-click here</ContextMenuTrigger>
      {children}
    </ContextMenu>
  )
}

// menubarRef is the reference menubar; value names the open menu.
function menubarRef(value: string): ReactNode {
  return (
    <Menubar value={value}>
      <MenubarMenu value="file">
        <MenubarTrigger>File</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>
            New tab <MenubarShortcut>⌘T</MenubarShortcut>
          </MenubarItem>
          <MenubarItem>New window</MenubarItem>
          <MenubarSeparator />
          <MenubarItem>Documentation</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu value="view">
        <MenubarTrigger>View</MenubarTrigger>
        <MenubarContent>
          <MenubarCheckboxItem>Always show bookmarks bar</MenubarCheckboxItem>
          <MenubarCheckboxItem checked>Always show full URLs</MenubarCheckboxItem>
          <MenubarSeparator />
          <MenubarItem inset>
            Reload <MenubarShortcut>⌘R</MenubarShortcut>
          </MenubarItem>
          <MenubarItem inset disabled>
            Force reload
          </MenubarItem>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu value="profiles">
        <MenubarTrigger>Profiles</MenubarTrigger>
        <MenubarContent>
          <MenubarRadioGroup value="grace">
            <MenubarLabel inset>Profile</MenubarLabel>
            <MenubarRadioItem value="ada">Ada</MenubarRadioItem>
            <MenubarRadioItem value="grace">Grace</MenubarRadioItem>
          </MenubarRadioGroup>
          <MenubarSeparator />
          <MenubarItem inset variant="destructive">
            Remove profile
          </MenubarItem>
        </MenubarContent>
      </MenubarMenu>
    </Menubar>
  )
}

// navigationRef is the reference navigation menu; value names the open item.
// The Gx port has no measured viewport, so the reference uses the pinned
// viewport={false} mode. Radix marks an active link with an empty
// data-active, which the pinned data-[active=true] recipe never matches; the
// reference passes the value the recipe expects.
function navigationRef(value: string): ReactNode {
  return (
    <NavigationMenu viewport={false} value={value}>
      <NavigationMenuList>
        <NavigationMenuItem value="products">
          <NavigationMenuTrigger>Products</NavigationMenuTrigger>
          <NavigationMenuContent>
            <ul className="grid w-48 gap-1">
              <li>
                <NavigationMenuLink href="/products">All products</NavigationMenuLink>
              </li>
              <li>
                <NavigationMenuLink href="/products/new">New arrivals</NavigationMenuLink>
              </li>
            </ul>
          </NavigationMenuContent>
        </NavigationMenuItem>
        <NavigationMenuItem>
          <NavigationMenuLink href="/" data-active="true" className={navigationMenuTriggerStyle()}>
            Home
          </NavigationMenuLink>
        </NavigationMenuItem>
        <NavigationMenuItem>
          <NavigationMenuLink href="/docs" className={navigationMenuTriggerStyle()}>
            Docs
          </NavigationMenuLink>
        </NavigationMenuItem>
      </NavigationMenuList>
    </NavigationMenu>
  )
}

export type Ref = {
  // body is the reference rendering, or a function of the open flag when an
  // overlay must be shown for the capture.
  body: ReactNode | ((open: boolean) => ReactNode)
  // focus is the CSS selector of the element whose focus state the spec
  // captures. Omitted when the fixture has no focus target.
  focus?: string
}

export const refs: Record<string, Ref> = {
  button: { body: <Button>Button</Button>, focus: "button" },
  badge: { body: <Badge>Badge</Badge> },
  alert: {
    body: (
      <Alert>
        <AlertTitle>Heads up</AlertTitle>
        <AlertDescription>You can add components to your app.</AlertDescription>
      </Alert>
    ),
  },
  card: {
    body: (
      <Card>
        <CardHeader>
          <CardTitle>Card title</CardTitle>
          <CardDescription>A short description.</CardDescription>
        </CardHeader>
        <CardContent>Card body.</CardContent>
        <CardFooter>Footer</CardFooter>
      </Card>
    ),
  },
  input: { body: <Input placeholder="Email" />, focus: "input" },
  textarea: { body: <Textarea placeholder="Tell us more." />, focus: "textarea" },
  label: { body: <Label htmlFor="email">Email</Label> },
  separator: { body: <Separator className="w-40" /> },
  skeleton: { body: <Skeleton className="h-4 w-40" /> },
  kbd: { body: <Kbd>K</Kbd> },
  spinner: { body: <Spinner /> },
  progress: { body: <Progress value={50} aria-label="Upload" /> },
  avatar: {
    body: (
      <Avatar>
        <AvatarFallback>NA</AvatarFallback>
      </Avatar>
    ),
  },
  "aspect-ratio": {
    body: (
      <AspectRatio ratio={16 / 9} className="rounded-md">
        16 / 9
      </AspectRatio>
    ),
  },
  checkbox: {
    body: (
      <div className="flex items-center gap-2">
        <Checkbox id="terms" name="terms" />
        <Label htmlFor="terms">Accept the terms</Label>
      </div>
    ),
    focus: "button[role=checkbox]",
  },
  switch: { body: <Switch name="wifi" aria-label="Wifi" />, focus: "button[role=switch]" },
  "radio-group": {
    body: (
      <RadioGroup defaultValue="free">
        <div className="flex items-center gap-2">
          <RadioGroupItem value="free" id="free" />
          <Label htmlFor="free">Free</Label>
        </div>
        <div className="flex items-center gap-2">
          <RadioGroupItem value="pro" id="pro" />
          <Label htmlFor="pro">Pro</Label>
        </div>
      </RadioGroup>
    ),
    focus: "button[role=radio]",
  },
  toggle: { body: <Toggle name="bold">Bold</Toggle>, focus: "button" },
  "toggle-group": {
    body: (
      <ToggleGroup type="single" defaultValue="left">
        <ToggleGroupItem value="left">Left</ToggleGroupItem>
        <ToggleGroupItem value="center">Center</ToggleGroupItem>
        <ToggleGroupItem value="right">Right</ToggleGroupItem>
      </ToggleGroup>
    ),
    focus: "button",
  },
  table: {
    body: (
      <Table>
        <TableCaption>A list of invoices.</TableCaption>
        <TableHeader>
          <TableRow>
            <TableHead>Invoice</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Amount</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>INV-001</TableCell>
            <TableCell>Paid</TableCell>
            <TableCell>$120.00</TableCell>
          </TableRow>
          <TableRow>
            <TableCell>INV-002</TableCell>
            <TableCell>Open</TableCell>
            <TableCell>$80.00</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    ),
  },
  breadcrumb: {
    body: (
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink href="/">Home</BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbLink href="/docs">Docs</BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>Breadcrumb</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>
    ),
  },
  pagination: {
    body: (
      <Pagination>
        <PaginationContent>
          <PaginationItem>
            <PaginationPrevious href="/?page=1" />
          </PaginationItem>
          <PaginationItem>
            <PaginationLink href="/?page=1" isActive>
              1
            </PaginationLink>
          </PaginationItem>
          <PaginationItem>
            <PaginationLink href="/?page=2">2</PaginationLink>
          </PaginationItem>
          <PaginationItem>
            <PaginationEllipsis />
          </PaginationItem>
          <PaginationItem>
            <PaginationNext href="/?page=2" />
          </PaginationItem>
        </PaginationContent>
      </Pagination>
    ),
  },
  empty: {
    body: (
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant="icon">+</EmptyMedia>
          <EmptyTitle>No projects</EmptyTitle>
          <EmptyDescription>Create your first project to start.</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>New project</EmptyContent>
      </Empty>
    ),
  },
  item: {
    body: (
      <Item className="p-4">
        <ItemMedia>*</ItemMedia>
        <ItemContent>
          <ItemTitle>Item title</ItemTitle>
          <ItemDescription>A short description of the item.</ItemDescription>
        </ItemContent>
        <ItemActions>Open</ItemActions>
      </Item>
    ),
  },
  "input-group": {
    body: (
      <InputGroup>
        <InputGroupAddon>
          <InputGroupText>$</InputGroupText>
        </InputGroupAddon>
        <InputGroupInput placeholder="0.00" />
      </InputGroup>
    ),
  },
  field: {
    body: (
      <Field>
        <FieldLabel htmlFor="email">Email</FieldLabel>
        <FieldDescription>We never share your email.</FieldDescription>
      </Field>
    ),
  },
  "button-group": {
    body: (
      <ButtonGroup>
        <Button variant="outline">One</Button>
        <Button variant="outline">Two</Button>
        <Button variant="outline">Three</Button>
      </ButtonGroup>
    ),
    focus: "button",
  },
  collapsible: {
    body: (
      <Collapsible defaultOpen>
        <CollapsibleTrigger className="inline-flex cursor-pointer list-none items-center gap-2 rounded-md outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]">
          Show details
        </CollapsibleTrigger>
        <CollapsibleContent>Hidden content.</CollapsibleContent>
      </Collapsible>
    ),
    focus: "button",
  },
  accordion: {
    body: (
      <Accordion type="single" collapsible defaultValue="faq">
        <AccordionItem value="faq">
          <AccordionTrigger>Is it accessible?</AccordionTrigger>
          <AccordionContent>Yes. It uses the native details element.</AccordionContent>
        </AccordionItem>
        <AccordionItem value="faq2">
          <AccordionTrigger>Is it animated?</AccordionTrigger>
          <AccordionContent>No. The panel opens at once.</AccordionContent>
        </AccordionItem>
      </Accordion>
    ),
    focus: "button",
  },
  dialog: {
    body: (open) => (
      <Dialog open={open}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit profile</DialogTitle>
            <DialogDescription>Change your display name.</DialogDescription>
          </DialogHeader>
          <div className="text-sm">Dialog body.</div>
          <DialogFooter>
            <Button>Save</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    ),
  },
  "alert-dialog": {
    body: (open) => (
      <AlertDialog open={open}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this item?</AlertDialogTitle>
            <AlertDialogDescription>This action cannot be undone.</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive">Delete</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    ),
  },
  sheet: {
    body: (open) => (
      <Sheet open={open}>
        <SheetContent side="right">
          <SheetHeader>
            <SheetTitle>Filters</SheetTitle>
            <SheetDescription>Narrow the result set.</SheetDescription>
          </SheetHeader>
          <div className="flex-1 overflow-y-auto px-4 text-sm">Sheet body.</div>
        </SheetContent>
      </Sheet>
    ),
  },
  drawer: {
    body: (open) => (
      <Drawer open={open}>
        <DrawerContent>
          <DrawerHeader>
            <DrawerTitle>Share this page</DrawerTitle>
            <DrawerDescription>Choose a destination.</DrawerDescription>
          </DrawerHeader>
          <div className="p-4 pt-0 text-sm">Drawer body.</div>
          <DrawerFooter>
            <Button>Copy link</Button>
          </DrawerFooter>
        </DrawerContent>
      </Drawer>
    ),
  },
  popover: {
    body: (open) => (
      <Popover open={open}>
        <PopoverContent>
          <PopoverHeader>
            <PopoverTitle>Dimensions</PopoverTitle>
            <PopoverDescription>Set the dimensions for the layer.</PopoverDescription>
          </PopoverHeader>
        </PopoverContent>
      </Popover>
    ),
  },
  "dropdown-menu": {
    body: (open) => (
      <DropdownMenu open={open}>
        <DropdownMenuContent className="w-56">
          <DropdownMenuLabel>My account</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem>Profile</DropdownMenuItem>
            <DropdownMenuItem>
              Settings
              <DropdownMenuShortcut>⌘S</DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem>Documentation</DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuCheckboxItem checked>Status bar</DropdownMenuCheckboxItem>
          <DropdownMenuCheckboxItem>Panel</DropdownMenuCheckboxItem>
          <DropdownMenuSeparator />
          <DropdownMenuRadioGroup value="top">
            <DropdownMenuRadioItem value="top">Top</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="bottom">Bottom</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive">Sign out</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    ),
  },
  "context-menu": {
    body: (open) => (
      <OpenContextMenu open={open}>
        <ContextMenuContent className="w-52">
          <ContextMenuItem>
            Copy
            <ContextMenuShortcut>⌘C</ContextMenuShortcut>
          </ContextMenuItem>
          <ContextMenuItem>Cut</ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuCheckboxItem checked>Show bookmarks</ContextMenuCheckboxItem>
          <ContextMenuSeparator />
          <ContextMenuRadioGroup value="ada">
            <ContextMenuLabel inset>People</ContextMenuLabel>
            <ContextMenuRadioItem value="ada">Ada</ContextMenuRadioItem>
            <ContextMenuRadioItem value="grace">Grace</ContextMenuRadioItem>
          </ContextMenuRadioGroup>
          <ContextMenuSeparator />
          <ContextMenuItem>Docs</ContextMenuItem>
          <ContextMenuItem variant="destructive">Delete</ContextMenuItem>
        </ContextMenuContent>
      </OpenContextMenu>
    ),
  },
  tooltip: {
    body: (open) => (
      <TooltipProvider>
        <Tooltip open={open}>
          <TooltipTrigger className="text-sm">Hover me</TooltipTrigger>
          <TooltipContent>Add to library</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    ),
  },
  "hover-card": {
    body: (open) => (
      <HoverCard open={open}>
        <HoverCardTrigger className="text-sm">@ada</HoverCardTrigger>
        <HoverCardContent className="text-sm">
          <p className="font-medium">Ada Lovelace</p>
          <p className="text-muted-foreground">First programmer.</p>
        </HoverCardContent>
      </HoverCard>
    ),
  },
  menubar: { body: menubarRef("") },
  "menubar-menu": { body: (open) => menubarRef(open ? "view" : "") },
  "navigation-menu": { body: navigationRef(""), focus: "a" },
  "navigation-menu-content": { body: (open) => navigationRef(open ? "products" : "") },
  select: {
    body: (
      <Select value="free">
        <SelectTrigger className="w-48">
          <SelectValue>Free</SelectValue>
        </SelectTrigger>
      </Select>
    ),
    focus: "button",
  },
  "select-sm": {
    body: (
      <Select value="free">
        <SelectTrigger size="sm" className="w-48">
          <SelectValue>Free</SelectValue>
        </SelectTrigger>
      </Select>
    ),
  },
  tabs: {
    // The Gx tabs are underline tabs on native buttons; the pinned shadcn
    // tabs are pill tabs. The reference pins the Gx design.
    body: (
      <div className="w-full" data-gx-tabs>
        <div>
          <button
            type="button"
            data-selected="true"
            className="-mb-px border-b-2 border-foreground px-3 py-1.5 text-sm font-medium text-foreground"
          >
            Account
          </button>
          <button
            type="button"
            className="-mb-px border-b-2 border-transparent px-3 py-1.5 text-sm font-medium text-muted-foreground"
          >
            Password
          </button>
        </div>
        <div className="pt-4" data-gx-tab-panel>
          Account settings.
        </div>
      </div>
    ),
  },
  "scroll-area": {
    // The Gx scroll area is a native scroller with a thin scrollbar.
    body: (
      <div
        className="relative h-24 w-48 overflow-auto rounded-md border border-border p-2"
        tabIndex={0}
        style={{ scrollbarWidth: "thin" }}
      >
        Line one. Line two. Line three. Line four. Line five. Line six. Line seven. Line eight.
      </div>
    ),
  },
  slider: {
    // The Gx slider is a native range input with the shadcn tokens.
    body: <input type="range" min={0} max={100} defaultValue={50} aria-label="Volume" className="h-2 w-full cursor-pointer appearance-none rounded-full bg-muted accent-primary" />,
  },
  sidebar: {
    // The Gx sidebar is a static aside; the pinned shadcn sidebar is an
    // interactive composition. The reference pins the Gx design.
    body: (
      <aside id="demo-sidebar" className="flex h-72 w-64 shrink-0 flex-col border-r border-border bg-sidebar text-sidebar-foreground">
        <div className="flex items-center gap-2 p-4">Gx</div>
        <div className="flex-1 overflow-y-auto p-2">
          <div className="flex flex-col gap-1 py-2">
            <p className="px-2 text-xs font-medium text-muted-foreground">Menu</p>
            <a href="/" aria-current="page" className="flex items-center gap-2 rounded-md bg-sidebar-accent px-2 py-1.5 text-sm text-sidebar-accent-foreground no-underline">
              Home
            </a>
            <a href="/docs" className="flex items-center gap-2 rounded-md px-2 py-1.5 text-sm no-underline">
              Docs
            </a>
          </div>
        </div>
        <div className="mt-auto p-4">v0.1.0</div>
      </aside>
    ),
  },
  toast: {
    // The toaster is the server-pushed toast region (Gx-native, REQ-REG-11).
    // The toast has the look of the shadcn sonner toast on the same tokens.
    body: (
      <div id="gx-toaster" role="region" aria-label="Notifications" aria-live="polite" className="static right-4 bottom-4 z-50 flex w-[356px] max-w-[calc(100vw-2rem)] flex-col gap-3">
        <div role="status" data-gx-toast className="flex w-full items-start gap-2 rounded-lg border border-border bg-popover p-4 text-sm text-popover-foreground shadow-lg">
          <div className="grid min-w-0 flex-1 gap-0.5">
            <div className="leading-5 font-medium">Saved</div>
          </div>
          <button type="button" aria-label="Close" className="-my-0.5 -mr-1 inline-flex size-6 shrink-0 items-center justify-center rounded-md text-muted-foreground outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50">
            <XIcon className="size-4" />
          </button>
        </div>
      </div>
    ),
  },
}
